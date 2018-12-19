package utils

import (
	"bytes"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"gopkg.in/src-d/enry.v1"
	"gopkg.in/src-d/enry.v1/data"
	"io"
	"strings"
)

var OdtType = filetype.NewType("odt", "application/vnd.oasis.opendocument.text")
var PlainType = filetype.NewType("txt", "text/plain")
var IcalType = filetype.NewType("ics", "text/calendar")
var MarkdownType = filetype.NewType("md", "text/markdown")
var RestType = filetype.NewType("rst", "text/x-rst")
var HTMLType = filetype.NewType("html", "text/html")
var icalBegin = []byte("BEGIN:VCALENDAR")

func init() {
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

func icalMatcher(buf []byte) bool {
	if len(buf) < 28 {
		return false
	}
	return bytes.Contains(buf[:28], icalBegin)
}

func GuessReader(filename string, reader io.Reader) (types.Type, io.Reader, error) {
	b := new(bytes.Buffer)
	b.Grow(8192)
	buffer := make([]byte, 8192)
	_, err := io.ReadFull(io.TeeReader(reader, b), buffer)
	reader = io.MultiReader(b, reader)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return types.Unknown, reader, err
	}

	t, err := Guess(filename, buffer)
	return t, reader, err
}

func Guess(filename string, content []byte) (types.Type, error) {
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
		mime, ext := mimetype.Detect(content)
		if t2 := m2f(mime, ext); t2 != types.Unknown {
			return t2, nil
		}
		return t, nil
	}

	if t != types.Unknown && t != matchers.TypeEot && t != PlainType {
		return t, nil
	}

	mime, ext := mimetype.Detect(content)
	if t2 := m2f(mime, ext); t2 != types.Unknown {
		return t2, nil
	}
	if filename != "" {
		if langs := getLanguages(filename, content); len(langs) > 0 {
			if mime, ok := data.LanguagesMime[langs[0]]; ok {
				if exts := data.ExtensionsByLanguage[langs[0]]; len(exts) > 0 {
					return m2f(mime, exts[0]), nil
				}
			}
		}
	}
	if IsBinary(content) {
		return matchers.TypeEot, nil
	}
	if icalMatcher(content) {
		return IcalType, nil
	}
	return PlainType, nil
}

func m2f(t, ext string) types.Type {
	ext = strings.Trim(ext, ".")
	if t == "" || t == "application/octet-stream" || t == "text/plain" {
		return types.Unknown
	}
	if f := types.Get(ext); f != types.Unknown && f != matchers.TypeEot && f != PlainType {
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

func getLanguages(filename string, content []byte) []string {
	var languages []string
	var candidates []string
	for _, strategy := range enry.DefaultStrategies {
		languages = strategy(filename, content, candidates)
		if len(languages) == 1 {
			return languages
		}

		if len(languages) > 0 {
			candidates = append(candidates, languages...)
		}
	}

	return languages
}
