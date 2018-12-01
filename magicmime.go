package main

import (
	"bytes"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"io"
)

func Guess(reader io.Reader) (types.Type, io.Reader, error) {
	b := new(bytes.Buffer)
	b.Grow(8192)
	wrappedReader := io.TeeReader(reader, b)
	t, err := filetype.MatchReader(wrappedReader)
	return t, io.MultiReader(b, reader), err
}
