package extractors

import (
	"errors"
	"fmt"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-cmd/cmd"
)

var dateFormat = "Mon Jan _2 15:04:05 2006 MST"

// Sat Nov 24 21:13:01 2018 CET
// Tue Nov  6 01:57:57 2018 CET


type AbsentExe struct {
	Err error
	Exe string
}

func (err *AbsentExe) Error() string {
	return fmt.Sprintf("Executable '%s' is not in PATH: %s", err.Exe, err.Err)
}

func Absent(exe string, err error) error {
	return &AbsentExe{Exe: exe, Err: err}
}

func PDFToText(filename string) (string, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	pdftotextPath, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", Absent("pdftotext", err)
	}
	command := cmd.NewCmd(pdftotextPath, "-nopgbrk", "-eol", "unix", "-q", "-enc", "UTF-8", filename, "-")
	status := <-command.Start()
	if status.Error != nil {
		return "", status.Error
	}
	if !status.Complete {
		return "", errors.New("pdftotext stopped abnormaly")
	}
	if status.Exit != 0 {
		return "", fmt.Errorf("pdftotext exit code not null: %d", status.Exit)
	}
	return strings.Join(status.Stdout, "\n"), nil
}

func PDFBytesToText(content []byte) (result string, err error) {
	temp, err := utils.NewTempFile(content)
	if err != nil {
		return "", err
	}
	_ = temp.RemoveAfter(func(name string) error {
		result, err = PDFToText(name)
		return err
	})
	return result, err
}

func PDFInfo(filename string) (*models.PDFMeta, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	pdfinfoPath, err := exec.LookPath("pdfinfo")
	if err != nil {
		return nil, Absent("pdfinfo", err)
	}
	command := cmd.NewCmd(pdfinfoPath, filename)
	status := <-command.Start()
	if status.Error != nil {
		return nil, status.Error
	}
	if !status.Complete {
		return nil, errors.New("pdfinfo stopped abnormaly")
	}
	if status.Exit != 0 {
		return nil, fmt.Errorf("pdfinfo exit code not null: %d", status.Exit)
	}
	meta := new(models.PDFMeta)
	for _, line := range status.Stdout {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "title":
			meta.Title = value
		case "author":
			meta.Author = value
		case "producer":
			meta.Producer = value
		case "creationdate":
			t, err := time.Parse(dateFormat, value)
			if err == nil {
				meta.CreationDate = &t
			}
		case "moddate":
			t, err := time.Parse(dateFormat, value)
			if err == nil {
				meta.ModDate = &t
			}
		case "tagged":
			meta.Tagged = value != "no"
		case "userproperties":
			meta.UserProperties = value != "no"
		case "form":
			meta.Form = value
		case "javascript":
			meta.Javascript = value != "no"
		case "pages":
			p, err := strconv.ParseInt(value, 10, 32)
			if err == nil {
				meta.Pages = int(p)
			}
		case "encrypted":
			meta.Encrypted = value != "no"
		case "page size":
			meta.PageSize = value
		case "file size":
			value = strings.TrimSpace(strings.TrimSuffix(value, "bytes"))
			s, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				meta.FileSize = int(s)
			}
		case "optimized":
			meta.Optimized = value != "no"
		case "pdf version":
			meta.Version = value
		default:
		}
	}
	return meta, nil
}

func PDFBytesInfo(pdf []byte) (meta *models.PDFMeta, err error) {
	temp, err := utils.NewTempFile(pdf)
	if err != nil {
		return nil, err
	}
	_ = temp.RemoveAfter(func(name string) error {
		meta, err = PDFInfo(name)
		return err
	})
	return meta, err
}
