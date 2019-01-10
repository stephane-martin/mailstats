package phishtank

import (
	"compress/bzip2"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/plar/go-adaptive-radix-tree"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var lock sync.Mutex

type Tree struct {
	art.Tree
}

func (t *Tree) Get(url string) []*models.PhishtankEntry {
	e, ok := t.Search([]byte(url))
	if !ok {
		return nil
	}
	return e.([]*models.PhishtankEntry)
}

func (t *Tree) Walk(f func(string, []*models.PhishtankEntry) bool) {
	t.ForEach(func(node art.Node) bool {
		if node.Kind() == art.Leaf {
			return f(string(node.Key()), node.Value().([]*models.PhishtankEntry))
		}
		return true
	})
}

func BuildTree(ctx context.Context, entries chan *models.PhishtankEntry, errs chan error, logger log15.Logger) (*Tree, error) {
	tree := art.New()

	logger.Info("Building phishtank tree")
	defer logger.Info("Finished phishtank tree")
L:
	for {
		if errs == nil && entries == nil {
			return &Tree{Tree: tree}, nil
		}
		select {
		case entry, ok := <-entries:
			if !ok {
				entries = nil
				continue L
			}
			e, ok := tree.Search([]byte(entry.URL))
			if !ok {
				tree.Insert([]byte(entry.URL), []*models.PhishtankEntry{entry})
			} else {
				en, ok := e.([]*models.PhishtankEntry)
				if !ok {
					return nil, errors.New("corrupted tree")
				}
				en = append(en, entry)
				tree.Insert([]byte(entry.URL), en)
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue L
			}
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func parseFile(r io.Reader, entries chan *models.PhishtankEntry, errs chan error) {
	decoder := xml.NewDecoder(bzip2.NewReader(r))
	for {
		token, err := decoder.Token()
		if token == nil || err == io.EOF {
			return
		}
		if err != nil {
			errs <- err
			return
		}
		if t, ok := token.(xml.StartElement); ok {
			if t.Name.Local == "entry" {
				var entry Entry
				err := decoder.DecodeElement(&entry, &t)
				if err == nil {
					gEntry := entry.Parse()
					if gEntry != nil {
						entries <- gEntry
					}
				}
			}
		}
	}
}

func Download(ctx context.Context, cacheDir string, applicationKey string, logger log15.Logger) (chan *models.PhishtankEntry, chan error) {
	lock.Lock()
	entries := make(chan *models.PhishtankEntry)
	errs := make(chan error)

	cacheDir = filepath.Join(cacheDir, "phishtank")
	d, err := os.Open(cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			errs <- err
			close(errs)
			close(entries)
			lock.Unlock()
			return entries, errs
		}
		err := os.MkdirAll(cacheDir, 0755)
		if err != nil {
			errs <- err
			close(errs)
			close(entries)
			lock.Unlock()
			return entries, errs
		}
	}
	_ = d.Close()

	applicationKey = strings.TrimSpace(applicationKey)
	if applicationKey == "" {
		errs <- errors.New("No phishtank application key provided")
		close(errs)
		close(entries)
		lock.Unlock()
		return entries, errs
	}

	url := fmt.Sprintf("http://data.phishtank.com/data/%s/online-valid.xml.bz2", applicationKey)

	previousETag := ""
	feedFilename := filepath.Join(cacheDir, "feed")
	etagFilename := filepath.Join(cacheDir, "etag")

	etagFile, err := os.Open(etagFilename)
	if err != nil {
		if !os.IsNotExist(err) {
			errs <- err
			close(errs)
			close(entries)
			lock.Unlock()
			return entries, errs
		}
		logger.Info("No ETag for Phishtank feed on filesystem")
	} else {
		content, err := ioutil.ReadAll(etagFile)
		_ = etagFile.Close()
		if err != nil {
			errs <- err
			close(errs)
			close(entries)
			lock.Unlock()
			return entries, errs
		}
		previousETag = string(content)
		logger.Info("Previous Phishtank ETag", "etag", previousETag)
	}

	client := utils.NewHTTPClient(10*time.Second, 1, 3)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		errs <- err
		close(errs)
		close(entries)
		lock.Unlock()
		return entries, errs
	}
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		errs <- err
		close(errs)
		close(entries)
		lock.Unlock()
		return entries, errs
	}
	defer resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		errs <- errors.New("phishtank HEAD response does not have a ETag header")
		close(errs)
		close(entries)
		lock.Unlock()
		return entries, errs
	}
	logger.Info("New ETag for Phishtank", "etag", etag)

	if etag == previousETag {
		logger.Info("Previous and new ETags for Phishtank are same")
		feedFile, err := os.Open(feedFilename)
		if err != nil {
			if !os.IsNotExist(err) {
				errs <- err
				close(errs)
				close(entries)
				lock.Unlock()
				return entries, errs
			}
			logger.Info("No cached Phishtank feed on filesystem")
		} else {
			logger.Info("Using cached Phishtank feed")
			go func() {
				parseFile(feedFile, entries, errs)
				close(errs)
				close(entries)
				_ = feedFile.Close()
				lock.Unlock()
			}()
			return entries, errs
		}
	}

	go func() {
		defer func() {
			close(errs)
			close(entries)
			lock.Unlock()
		}()
		client := utils.NewHTTPClient(3*time.Minute, 1, 3)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			errs <- err
			return
		}
		req = req.WithContext(ctx)
		resp, err := client.Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			errs <- fmt.Errorf("HTTP status code not OK: %d", resp.StatusCode)
			return
		}
		feedFile, err := os.OpenFile(feedFilename, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			errs <- err
			return
		}
		_, err = io.Copy(feedFile, resp.Body)
		_ = feedFile.Close()
		if err != nil {
			errs <- err
			return
		}

		etag := resp.Header.Get("ETag")
		if etag != "" {
			etagFile, err := os.OpenFile(etagFilename, os.O_WRONLY|os.O_CREATE, 0644)
			if err == nil {
				_, err := etagFile.WriteString(etag)
				_ = etagFile.Close()
				if err != nil {
					logger.Warn("Error writing ETag file", "error", err)
				}
			} else {
				logger.Warn("Error opening ETag file", "error", err)
			}
		} else {
			logger.Warn("phishtank GET response does not have a ETag header")
		}

		feedFile, err = os.Open(feedFilename)
		if err != nil {
			errs <- err
			return
		}

		parseFile(feedFile, entries, errs)
		_ = feedFile.Close()
	}()

	return entries, errs
}
