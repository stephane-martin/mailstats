package extractors

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/go-cmd/cmd"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// taken from docconv project
// https://github.com/sajari/docconv

var AntiwordPath string

func init() {
	p, err := exec.LookPath("antiword")
	if err == nil {
		AntiwordPath = p
	}
}

func ConvertDocx(filename string) (string, map[string]interface{}, bool, error) {
	f, err := os.Open(filename)
	if err!= nil {
		return "", nil, false, err
	}
	//noinspection GoUnhandledErrorResult
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", nil, false, err
	}
	return ConvertBytesDocx(b)
}

func ConvertBytesDocx(b []byte) (content string, props map[string]interface{}, hasMacro bool, err error) {
	props = make(map[string]interface{})
	var headerFull, textBody, footerFull, header, footer string
	var zr *zip.Reader
	var rc io.ReadCloser

	zr, err = zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		err = fmt.Errorf("error unzipping data: %v", err)
		return
	}

	// Regular expression for XML files to include in the text parsing
	reHeaderFile, _ := regexp.Compile("^word/header[0-9]+.xml$")
	reFooterFile, _ := regexp.Compile("^word/footer[0-9]+.xml$")

	for _, f := range zr.File {
		switch {
		case f.Name == "word/vbaProject.bin":
			hasMacro = true

		case f.Name == "docProps/core.xml":
			rc, err = f.Open()
			if err != nil {
				err = fmt.Errorf("error opening '%v' from archive: %v", f.Name, err)
				return
			}

			props, err = XMLToMap(rc)
			_ = rc.Close()
			if err != nil {
				err = fmt.Errorf("error parsing '%v': %v", f.Name, err)
				return
			}

			if tmp, ok := props["modified"]; ok {
				if t, err := time.Parse(time.RFC3339, fmt.Sprintf("%s", tmp)); err == nil {
					props["modified_date"] = t.Format(time.RFC3339)
					delete(props, "modified")
				}
			}
			if tmp, ok := props["created"]; ok {
				if t, err := time.Parse(time.RFC3339, fmt.Sprintf("%s", tmp)); err == nil {
					props["created_date"] = t.Format(time.RFC3339)
					delete(props, "created")
				}
			}

			for k, v := range props {
				delete(props, k)
				props[utils.Snake(k)] = v
			}

		case f.Name == "word/document.xml":
			textBody, err = parseDocxText(f)
			if err != nil {
				return
			}

		case reHeaderFile.MatchString(f.Name):
			header, err = parseDocxText(f)
			if err != nil {
				return
			}
			headerFull += header + "\n"

		case reFooterFile.MatchString(f.Name):
			footer, err = parseDocxText(f)
			if err != nil {
				return
			}
			footerFull += footer + "\n"
		}
	}
	content = strings.TrimSpace(headerFull + "\n" + textBody + "\n" + footerFull)
	return
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
	return strings.TrimSpace(result), nil
}


// XMLToMap converts XML to a nested string map.
func XMLToMap(r io.Reader) (map[string]interface{}, error) {
	m := make(map[string]interface{})
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
			tagName = strings.TrimSpace(string(v.Name.Local))
		case xml.CharData:
			if tagName != "" {
				m[tagName] = strings.TrimSpace(string(v))
			}
		}
	}
	return m, nil
}

func ConvertDoc(filename string) (string, error) {
	if AntiwordPath == "" {
		return "", Absent("antiword")
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	command := cmd.NewCmd(AntiwordPath, filename)
	status := <-command.Start()
	if status.Error != nil {
		return "", status.Error
	}
	if !status.Complete {
		return "", errors.New("antiword stopped abnormaly")
	}
	if status.Exit != 0 {
		return "", fmt.Errorf("antiword exit code not null: %d", status.Exit)
	}
	return strings.TrimSpace(strings.Join(status.Stdout, "\n")), nil
}

func ConvertBytesDoc(b []byte) (content string, err error) {
	temp, err := utils.NewTempFile(b)
	if err != nil {
		return "", err
	}
	_ = temp.RemoveAfter(func(name string) error {
		content, err = ConvertDoc(name)
		return err
	})
	return content, err
}

func ConvertODT(filename string) (string, map[string]interface{}, error) {
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

func ConvertBytesODT(b []byte) (content string, props map[string]interface{}, err error) {
	props = make(map[string]interface{})
	var zr *zip.Reader
	var rc io.ReadCloser

	zr, err = zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		err = fmt.Errorf("error unzipping data: %v", err)
		return
	}

	for _, f := range zr.File {
		switch f.Name {
		case "meta.xml":
			rc, err = f.Open()
			if err != nil {
				err = fmt.Errorf("error extracting '%v' from archive: %v", f.Name, err)
				return
			}

			props, err = XMLToMap(rc)
			_ = rc.Close()
			if err != nil {
				err = fmt.Errorf("error parsing '%v': %v", f.Name, err)
				return
			}

			if tmp, ok := props["date"]; ok {
				if t, err := time.Parse("2006-01-02T15:04:05", fmt.Sprintf("%s", tmp)); err == nil {
					props["modified_date"] = t.Format(time.RFC3339)
					delete(props, "date")
				}
			}
			if tmp, ok := props["creation-date"]; ok {
				if t, err := time.Parse("2006-01-02T15:04:05", fmt.Sprintf("%s", tmp)); err == nil {
					props["created_date"] = t.Format(time.RFC3339)
					delete(props, "creation-date")
				}
			}

			for k, v := range props {
				delete(props, k)
				props[utils.Snake(k)] = v
			}

		case "content.xml":
			rc, err = f.Open()
			if err != nil {
				err = fmt.Errorf("error extracting '%v' from archive: %v", f.Name, err)
				return
			}

			content, err = XMLToText(rc, []string{"br", "p", "tab"}, []string{}, true)
			_ = rc.Close()

			if err != nil {
				err = fmt.Errorf("error parsing '%v': %v", f.Name, err)
				return
			}
		}
	}

	return
}