package main

import (
	"golang.org/x/text/unicode/norm"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"github.com/oklog/ulid"
	"time"
)

var rsource rand.Source
var rrand *rand.Rand
var mono io.Reader
var ulidChan chan ulid.ULID

func init() {
	rsource = rand.NewSource(1)
	rrand = rand.New(rsource)
	mono = ulid.Monotonic(rrand, 0)
	ulidChan = make(chan ulid.ULID)

	go func() {
		for {
			ulidChan <- ulid.MustNew(ulid.Timestamp(time.Now()), mono)
		}
	}()
}

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
	err = autoclose(f, func() error {
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

func normalize(s string) string {
	return norm.NFKC.String(s)
}

func NewULID() ulid.ULID {
	return <-ulidChan
}

func autoclose(w io.Closer, f func() error) (err error) {
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

func IsTimeout(e error) bool {
	if err, ok := e.(iTimeout); ok {
		return err.Timeout()
	}
	return false
}


func IsTemp(e error) bool {
	if err, ok := e.(iTemporary); ok {
		return err.Temporary()
	}
	return false
}

type iTimeout interface {
	Timeout() bool
}

type iTemporary interface {
	Temporary() bool
}