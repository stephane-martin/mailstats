package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-cmd/cmd"
	"github.com/urfave/cli"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var dateFormat = "Mon Jan _2 15:04:05 2006 MST"
// Sat Nov 24 21:13:01 2018 CET
// Tue Nov  6 01:57:57 2018 CET

func PDFInfoAction(c *cli.Context) error {
	filename := c.String("filename")
	if filename == "" {
		return cli.NewExitError("No filename", 1)
	}
	meta, err := PDFInfo(filename)
	if err != nil {
		return cli.NewExitError(err, 2)
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return cli.NewExitError(err, 2)
	}
	fmt.Println(string(b))
	return nil
}

type PDFMeta struct {
	Title          string     `json:"title,omitempty"`
	Author         string     `json:"author,omitempty"`
	Creator        string     `json:"creator,omitempty"`
	Producer       string     `json:"producer,omitempty"`
	CreationDate   *time.Time `json:"creation_date,omitempty"`
	ModDate        *time.Time `json:"mod_date,omitempty"`
	Tagged         bool       `json:"tagged"`
	UserProperties bool       `json:"user_properties"`
	Form           string     `json:"form,omitempty"`
	Javascript     bool       `json:"javascript"`
	Pages          int        `json:"pages"`
	Encrypted      bool       `json:"encrypted"`
	PageSize       string     `json:"page_size,omitempty"`
	FileSize       int        `json:"file_size"`
	Optimized      bool       `json:"optimized"`
	Version        string     `json:"version,omitempty"`
}

func PDFInfo(filename string) (*PDFMeta, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	pdfinfoPath, err := exec.LookPath("pdfinfo")
	if err != nil {
		return nil, err
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
	meta := new(PDFMeta)
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

func PDFBytesInfo(pdf []byte) (*PDFMeta, error) {
	temp, err := NewTempFile(pdf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = temp.Remove() }()
	return PDFInfo(temp.Name())
}
