package collectors

import (
	"context"
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/tinylib/msgp/msgp"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type FileStore struct {
	root    string
	new     string
	tmp     string
	watcher *fsnotify.Watcher
	logger  log15.Logger
	lock    sync.Mutex
	cond    *sync.Cond
}

func getOrCreateDir(path string) (os.FileInfo, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.Mkdir(path, 0755)
		if err != nil {
			return nil, err
		}
		stat, err = os.Stat(path)
	}
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("is not a directory")
	}
	return stat, nil
}

func NewFileStore(root string, logger log15.Logger) (*FileStore, error) {
	s := new(FileStore)
	s.logger = logger
	s.cond = sync.NewCond(&s.lock)
	r, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	s.root = r
	s.new = filepath.Join(s.root, "new")
	s.tmp = filepath.Join(s.root, "tmp")
	_, err = getOrCreateDir(s.root)
	if err != nil {
		return nil, err
	}
	_, err = getOrCreateDir(s.new)
	if err != nil {
		return nil, err
	}
	_, err = getOrCreateDir(s.tmp)
	if err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	s.watcher = w
	err = s.watcher.Add(s.new)
	if err != nil {
		_ = s.watcher.Close()
		return nil, err
	}

	go func() {
	L:
		for {
			select {
			case event, ok := <-s.watcher.Events:
				if !ok {
					break L
				}
				if event.Op == fsnotify.Create {
					s.logger.Debug("Watcher event", "name", event.Name)
					s.lock.Lock()
					s.cond.Signal()
					s.lock.Unlock()
				}
			case err, ok := <-s.watcher.Errors:
				if !ok {
					break L
				}
				s.logger.Warn("Watcher reported error", "error", err)
			}
		}
	}()

	return s, nil
}

func (s *FileStore) Close() error {
	return s.watcher.Close()
}

func (s *FileStore) New(uid ulid.ULID, obj *models.IncomingMail) error {
	tmppath := filepath.Join(s.tmp, uid.String())
	dest := filepath.Join(s.new, uid.String())
	f, err := os.OpenFile(tmppath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	err = utils.Autoclose(f, func() error {
		return msgp.Encode(f, obj)
	})
	if err != nil {
		_ = os.Remove(tmppath)
		return err
	}
	err = os.Rename(tmppath, dest)
	if err != nil {
		_ = os.Remove(tmppath)
		return err
	}
	return nil
}

func (s *FileStore) readDirNew() (string, error) {
	d, err := os.Open(s.new)
	if err != nil {
		return "", err
	}
	names, err := d.Readdirnames(1)
	_ = d.Close()
	if err == io.EOF {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(s.new, names[0]), nil
}

func (s *FileStore) Get(stop <-chan struct{}, obj *models.IncomingMail) error {
	c := make(chan struct{})
	var err error
	go func() {
		err = s.get(stop, obj)
		close(c)
	}()
	select {
	case <-stop:
		return context.Canceled
	case <-c:
		return err
	}
}

func (s *FileStore) get(stop <-chan struct{}, obj *models.IncomingMail) error {
	s.lock.Lock()
	newFile := ""
	var err error
	for {
		newFile, err = s.readDirNew()
		if err != nil {
			s.lock.Unlock()
			return err
		}
		if newFile != "" {
			break
		}
		s.cond.Wait()
	}
	defer s.lock.Unlock()
	f, err := os.Open(newFile)
	if err != nil {
		return err
	}
	err = msgp.Decode(f, obj)
	if err != nil {
		// TODO: delete file ?
		_ = f.Close()
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	select {
	case <-stop:
		// oops, we just retrieved some work from FS, but the client is already gone
		// so let's transfer to another waiter
		s.cond.Signal()
		return context.Canceled
	default:
		err = os.Remove(newFile)
		if err != nil {
			return err
		}
		return nil
	}
}
