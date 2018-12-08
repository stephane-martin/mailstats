package utils

import (
	"bytes"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"io"
	"io/ioutil"
	"strings"
)

func GuessReader(reader io.Reader) (types.Type, io.Reader, error) {
	b := new(bytes.Buffer)
	b.Grow(8192)
	t, err := filetype.MatchReader(io.TeeReader(reader, b))
	reader = io.MultiReader(b, reader)

	if err != nil {
		return t, reader, err
	}

	if t == matchers.TypeZip || t == types.Unknown {
		b2 := new(bytes.Buffer)
		content, err := ioutil.ReadAll(io.TeeReader(reader, b2))
		reader = io.MultiReader(b2, reader)

		if err == nil {
			t2, ext := mimetype.Detect(content)
			t3 := m2f(t2, ext)
			if t3 != types.Unknown {
				return t3, reader, nil
			}
		}
	}

	return t, reader, nil
}

func Guess(content []byte) (types.Type, error) {
	t, err := filetype.Match(content)
	if err != nil {
		return t, err
	}
	if t == matchers.TypeZip || t == types.Unknown {
		t2, ext := mimetype.Detect(content)
		t3 := m2f(t2, ext)
		if t3 != types.Unknown {
			return t3, nil
		}
	}
	return t, nil
}

func m2f(t, ext string) types.Type {
	if t == "" {
		return types.Unknown
	}
	f := types.Get(ext)
	if f != types.Unknown {
		return f
	}
	parts := strings.SplitN(t, "/", 2)
	return types.Type{
		Extension: ext,
		MIME: types.MIME{
			Type: parts[0],
			Subtype: parts[1],
			Value: t,
		},
	}
}