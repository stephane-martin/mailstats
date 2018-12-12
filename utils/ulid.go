package utils

import (
	"github.com/oklog/ulid"
	"io"
	"math/rand"
	"time"
)

func NewULID() ulid.ULID {
	return <-ulidChan
}

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
