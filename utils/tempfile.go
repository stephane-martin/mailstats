package utils

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
)

type TempFile struct {
	name string
	l sync.Mutex
}


func NewTempFile(content []byte) (*TempFile, error) {
	f, err := ioutil.TempFile("", "mailstats-*")
	if err != nil {
		return nil, err
	}
	t := &TempFile{name: f.Name()}
	err = Autoclose(f, func() error {
		if len(content) == 0 {
			return nil
		}
		_, e := f.Write(content)
		return e
	})
	if err != nil {
		_ = os.Remove(t.name)
		return nil, err
	}
	return t, nil
}

func (t *TempFile) Name() string {
	return t.name
}

func (t *TempFile) Remove() (err error) {
	t.l.Lock()
	defer t.l.Unlock()
	if t.name != "" {
		err = os.Remove(t.name)
		t.name = ""
	}
	return err
}

func (t *TempFile) RemoveAfter(f func(name string) error) (err error) {
	defer func() {
		e := recover()
		errRemove := t.Remove()
		if e != nil {
			panic(e)
		}
		if err == nil {
			err = errRemove
		}
	}()
	return f(t.Name())
}

func Autoclose(w io.Closer, f func() error) (err error) {
	defer func() {
		e := recover()
		errClose := w.Close()
		if e != nil {
			panic(e)
		}
		if err == nil {
			err = errClose
		}
	}()
	return f()
}
