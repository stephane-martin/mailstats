package main

import (
	"encoding/json"
	"github.com/mostlygeek/go-exiftool"
	"os/exec"
	"runtime"
)

type ExifToolWrapper struct {
	tool *exiftool.Pool
}

func NewExifToolWrapper() (*ExifToolWrapper, error) {
	path, err := exec.LookPath("exiftool")
	if err != nil {
		return nil, err
	}
	t, err := exiftool.NewPool(path, runtime.NumCPU(), "-json")
	if err != nil {
		return nil, err
	}
	return &ExifToolWrapper{tool: t}, nil
}

func (w *ExifToolWrapper) ExtractFromFile(filename string) (map[string]interface{}, error) {
	res, err := w.tool.Extract(filename)
	if err != nil {
		return nil, err
	}
	attributes := []map[string]interface{}{make(map[string]interface{})}
	err = json.Unmarshal(res, &attributes)
	if err != nil {
		return nil, err
	}
	return attributes[0], nil
}

func (w *ExifToolWrapper) Extract(content []byte) (map[string]interface{}, error) {
	temp, err := NewTempFile(content)
	if err != nil {
		return nil, err
	}
	defer func() { _ = temp.Remove() }()
	return w.ExtractFromFile(temp.Name())
}

func (w *ExifToolWrapper) Close() error {
	w.tool.Stop()
	return nil
}