package extractors

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"time"
)

// taken from docconv project
// https://github.com/sajari/docconv

func ConvertDocx(filename string) (string, map[string]string, error) {
	f, err := os.Open(filename)
	if err!= nil {
		return "", nil, err
	}
	//noinspection GoUnhandledErrorResult
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", nil, err
	}
	return ConvertBytesDocx(b)
}

func ConvertBytesDocx(b []byte) (string, map[string]string, error) {
	meta := make(map[string]string)
	var textHeader, textBody, textFooter string

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", nil, fmt.Errorf("error unzipping data: %v", err)
	}

	// Regular expression for XML files to include in the text parsing
	reHeaderFile, _ := regexp.Compile("^word/header[0-9]+.xml$")
	reFooterFile, _ := regexp.Compile("^word/footer[0-9]+.xml$")

	for _, f := range zr.File {
		switch {
		case f.Name == "docProps/core.xml":
			rc, err := f.Open()
			if err != nil {
				return "", nil, fmt.Errorf("error opening '%v' from archive: %v", f.Name, err)
			}
			defer rc.Close()

			meta, err = XMLToMap(rc)
			if err != nil {
				return "", nil, fmt.Errorf("error parsing '%v': %v", f.Name, err)
			}

			if tmp, ok := meta["modified"]; ok {
				if t, err := time.Parse(time.RFC3339, tmp); err == nil {
					meta["ModifiedDate"] = fmt.Sprintf("%d", t.Unix())
				}
			}
			if tmp, ok := meta["created"]; ok {
				if t, err := time.Parse(time.RFC3339, tmp); err == nil {
					meta["CreatedDate"] = fmt.Sprintf("%d", t.Unix())
				}
			}

		case f.Name == "word/document.xml":
			textBody, err = parseDocxText(f)
			if err != nil {
				return "", nil, err
			}

		case reHeaderFile.MatchString(f.Name):
			header, err := parseDocxText(f)
			if err != nil {
				return "", nil, err
			}
			textHeader += header + "\n"

		case reFooterFile.MatchString(f.Name):
			footer, err := parseDocxText(f)
			if err != nil {
				return "", nil, err
			}
			textFooter += footer + "\n"
		}
	}

	return textHeader + "\n" + textBody + "\n" + textFooter, meta, nil
}

func parseDocxText(f *zip.File) (string, error) {
	r, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("error opening '%v' from archive: %v", f.Name, err)
	}
	defer r.Close()

	text, err := DocxXMLToText(r)
	if err != nil {
		return "", fmt.Errorf("error parsing '%v': %v", f.Name, err)
	}
	return text, nil
}

func DocxXMLToText(r io.Reader) (string, error) {
	return XMLToText(r, []string{"br", "p", "tab"}, []string{"instrText", "script"}, true)
}

func XMLToText(r io.Reader, breaks []string, skip []string, strict bool) (string, error) {
	var result string

	dec := xml.NewDecoder(r)
	dec.Strict = strict
	for {
		t, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch v := t.(type) {
		case xml.CharData:
			result += string(v)
		case xml.StartElement:
			for _, breakElement := range breaks {
				if v.Name.Local == breakElement {
					result += "\n"
				}
			}
			for _, skipElement := range skip {
				if v.Name.Local == skipElement {
					depth := 1
					for {
						t, err := dec.Token()
						if err != nil {
							// An io.EOF here is actually an error.
							return "", err
						}

						switch t.(type) {
						case xml.StartElement:
							depth++
						case xml.EndElement:
							depth--
						}

						if depth == 0 {
							break
						}
					}
				}
			}
		}
	}
	return result, nil
}


// XMLToMap converts XML to a nested string map.
func XMLToMap(r io.Reader) (map[string]string, error) {
	m := make(map[string]string)
	dec := xml.NewDecoder(r)
	var tagName string
	for {
		t, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch v := t.(type) {
		case xml.StartElement:
			tagName = string(v.Name.Local)
		case xml.CharData:
			m[tagName] = string(v)
		}
	}
	return m, nil
}

func ConvertODT(filename string) (string, map[string]string, error) {
	f, err := os.Open(filename)
	if err!= nil {
		return "", nil, err
	}
	//noinspection GoUnhandledErrorResult
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", nil, err
	}
	return ConvertBytesODT(b)
}

func ConvertBytesODT(b []byte) (string, map[string]string, error) {
	meta := make(map[string]string)
	var textBody string

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", nil, fmt.Errorf("error unzipping data: %v", err)
	}

	for _, f := range zr.File {
		switch f.Name {
		case "meta.xml":
			rc, err := f.Open()
			if err != nil {
				return "", nil, fmt.Errorf("error extracting '%v' from archive: %v", f.Name, err)
			}
			defer rc.Close()

			info, err := XMLToMap(rc)
			if err != nil {
				return "", nil, fmt.Errorf("error parsing '%v': %v", f.Name, err)
			}

			if tmp, ok := info["creator"]; ok {
				meta["Author"] = tmp
			}
			if tmp, ok := info["date"]; ok {
				if t, err := time.Parse("2006-01-02T15:04:05", tmp); err == nil {
					meta["ModifiedDate"] = fmt.Sprintf("%d", t.Unix())
				}
			}
			if tmp, ok := info["creation-date"]; ok {
				if t, err := time.Parse("2006-01-02T15:04:05", tmp); err == nil {
					meta["CreatedDate"] = fmt.Sprintf("%d", t.Unix())
				}
			}

		case "content.xml":
			rc, err := f.Open()
			if err != nil {
				return "", nil, fmt.Errorf("error extracting '%v' from archive: %v", f.Name, err)
			}
			defer rc.Close()

			textBody, err = XMLToText(rc, []string{"br", "p", "tab"}, []string{}, true)
			if err != nil {
				return "", nil, fmt.Errorf("error parsing '%v': %v", f.Name, err)
			}
		}
	}

	return textBody, meta, nil
}