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

var PDFToTextPath string
var PDFInfoPath string

func init() {
	path, err := exec.LookPath("pdftotext")
	if err == nil {
		PDFToTextPath = path
	}
	path, err = exec.LookPath("pdfinfo")
	if err == nil {
		PDFInfoPath = path
	}
}

type AbsentUtil struct {
	Exe string
}

func (err *AbsentUtil) Error() string {
	return fmt.Sprintf("Executable '%s' is not in PATH", err.Exe)
}

func Absent(exe string) error {
	return &AbsentUtil{Exe: exe}
}

func PDFToText(filename string) (string, error) {
	if PDFToTextPath == "" {
		return "", Absent("pdftotext")
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	command := cmd.NewCmd(PDFToTextPath, "-nopgbrk", "-eol", "unix", "-q", "-enc", "UTF-8", filename, "-")
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
	return strings.TrimSpace(strings.Join(status.Stdout, "\n")), nil
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

func PDFInfo(filename string, meta *models.PDFMeta) (*models.PDFMeta, error) {
	if PDFInfoPath == "" {
		return meta, Absent("pdfinfo")
	}
	if meta == nil {
		meta = new(models.PDFMeta)
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return meta, err
	}
	if !stat.Mode().IsRegular() {
		return meta, errors.New("not a regular file")
	}
	command := cmd.NewCmd(PDFInfoPath, filename)
	status := <-command.Start()
	if status.Error != nil {
		return meta, status.Error
	}
	if !status.Complete {
		return meta, errors.New("pdfinfo stopped abnormaly")
	}
	if status.Exit != 0 {
		return meta, fmt.Errorf("pdfinfo exit code not null: %d", status.Exit)
	}
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
				meta.Pages = int64(p)
			}
		case "encrypted":
			meta.Encrypted = value != "no"
		case "page size":
			meta.PageSize = value
		case "file size":
			value = strings.TrimSpace(strings.TrimSuffix(value, "bytes"))
			s, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				meta.FileSize = int64(s)
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

func PDFBytesInfo(pdf []byte, meta *models.PDFMeta) (*models.PDFMeta, error) {
	temp, err := utils.NewTempFile(pdf)
	if err != nil {
		return meta, err
	}
	_ = temp.RemoveAfter(func(name string) error {
		meta, err = PDFInfo(name, meta)
		return err
	})
	return meta, err
}
