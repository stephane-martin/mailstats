package parser

import (
	"encoding/base64"
	"github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"io"
	"io/ioutil"
	"mime"
	"mime/quotedprintable"
	"strings"
)

// WordDecode decodes word if necessary and returns decoded data
func WordDecode(word string) (string, error) {
	// return word unchanged if not RFC 2047 encoded
	if !strings.HasPrefix(word, "=?") || !strings.HasSuffix(word, "?=") || strings.Count(word, "?") != 4 {
		return word, nil
	}
	decoder := new(mime.WordDecoder)
	decoder.CharsetReader = charset.Reader
	return decoder.Decode(word)
}

// StringDecode splits text to list of words, decodes
// every word and assembles it back into a decoded string
func StringDecode(text string) (string, error) {
	words := strings.Split(text, " ")
	var err error
	for idx := range words {
		words[idx], err = WordDecode(words[idx])
		if err != nil {
			return "", err
		}
	}
	return strings.Join(words, " "), nil
}

func decodeLatin1(body io.Reader) (string, error) {
	r := charmap.ISO8859_15.NewDecoder().Reader(body)
	res, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func decodeUTF8(body io.Reader) (string, error) {
	r := unicode.UTF8.NewDecoder().Reader(body)
	res, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func decodeBody(body io.Reader, charset string, qp string) string {
	if qp == "quoted-printable" {
		body = quotedprintable.NewReader(body)
	} else if qp == "base64" {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}
	if charset != "" {
		encoder, err := htmlindex.Get(charset)
		if err == nil {
			r := encoder.NewDecoder().Reader(body)
			res, err := ioutil.ReadAll(r)
			if err == nil {
				return string(res)
			}
		}
	}
	res, err := decodeUTF8(body)
	if err == nil {
		return res
	}
	res, err = decodeLatin1(body)
	if err == nil {
		return res
	}
	// damn, nothing works
	return ""
}
