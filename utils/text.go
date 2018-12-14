package utils

import (
	"bytes"
	"encoding/json"
	"golang.org/x/text/unicode/norm"
	"io"
	"regexp"
	"strings"
)

func Normalize(s string) string {
	return norm.NFKC.String(s)
}

func JSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func JSONMarshal(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	err := JSONEncoder(&b).Encode(v)
	return b.Bytes(), err
}

func JSONString(v interface{}) (string, error) {
	var s strings.Builder
	enc := JSONEncoder(&s)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return s.String(), err
}

func Snake(s string) string {
	s = addWordBoundariesToNumbers(s)
	s = strings.Trim(s, " ")
	n := ""
	for i, v := range s {
		// treat acronyms as words, eg for JSONData -> JSON is a whole word
		nextCaseIsChanged := false
		if i+1 < len(s) {
			next := s[i+1]
			if (v >= 'A' && v <= 'Z' && next >= 'a' && next <= 'z') || (v >= 'a' && v <= 'z' && next >= 'A' && next <= 'Z') {
				nextCaseIsChanged = true
			}
		}

		if i > 0 && n[len(n)-1] != '_' && nextCaseIsChanged {
			// add underscore if next letter case type is changed
			if v >= 'A' && v <= 'Z' {
				n += "_" + string(v)
			} else if v >= 'a' && v <= 'z' {
				n += string(v) + "_"
			}
		} else if v == ' ' || v == '_' || v == '-' {
			// replace spaces/underscores with delimiters
			n += "_"
		} else {
			n = n + string(v)
		}
	}
	return strings.ToLower(n)
}


var numberSequence = regexp.MustCompile(`([a-zA-Z])(\d+)([a-zA-Z]?)`)
var numberReplacement = []byte(`$1 $2 $3`)

func addWordBoundariesToNumbers(s string) string {
	b := []byte(s)
	b = numberSequence.ReplaceAll(b, numberReplacement)
	return string(b)
}
