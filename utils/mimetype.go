package utils

import (
	"bytes"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"io"
	"strings"
)

var OdtType types.Type

func init() {
	OdtType = filetype.NewType("odt", "application/vnd.oasis.opendocument.text")
	filetype.AddMatcher(OdtType, odtMatcher)
}

func odtMatcher(buf []byte) bool {
	return len(buf) > 127 &&
		buf[0] == 0x50 &&
		buf[1] == 0x4B &&
		(buf[2] == 0x3 || buf[2] == 0x5 || buf[2] == 0x7) &&
		(buf[3] == 0x4 || buf[3] == 0x6 || buf[3] == 0x8) &&
		string(buf[30:38]) == "mimetype" &&
		string(buf[38:38+len(OdtType.MIME.Value)]) == OdtType.MIME.Value
}

func GuessReader(reader io.Reader) (types.Type, io.Reader, error) {
	b := new(bytes.Buffer)
	b.Grow(8192)
	buffer := make([]byte, 8192)
	_, err := io.ReadFull(io.TeeReader(reader, b), buffer)
	reader = io.MultiReader(b, reader)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return types.Unknown, reader, err
	}

	t, err := Guess(buffer)
	return t, reader, err
}

func Guess(content []byte) (types.Type, error) {
	t, err := filetype.Match(content)
	if err != nil {
		return t, err
	}
	if t == matchers.TypeZip {
		if matchers.Docx(content) {
			return matchers.TypeDocx, nil
		}
		if matchers.Pptx(content) {
			return matchers.TypePptx, nil
		}
		if matchers.Xlsx(content) {
			return matchers.TypeXlsx, nil
		}
		if odtMatcher(content) {
			return OdtType, nil
		}
	}
	if t == types.Unknown {
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
			Type:    parts[0],
			Subtype: parts[1],
			Value:   t,
		},
	}
}
