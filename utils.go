package main

import (
	"golang.org/x/text/unicode/norm"
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
	_, err = f.Write(content)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(t.name)
		return nil, err
	}
	err = f.Close()
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

func normalize(s string) string {
	return norm.NFKC.String(s)
}