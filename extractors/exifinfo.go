package extractors

import (
	"encoding/json"
	"github.com/mostlygeek/go-exiftool"
	"github.com/stephane-martin/mailstats/utils"
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

func (w *ExifToolWrapper) Extract(content []byte) (meta map[string]interface{}, err error) {
	temp, err := utils.NewTempFile(content)
	if err != nil {
		return nil, err
	}
	_ = temp.RemoveAfter(func(name string) error {
		meta, err = w.ExtractFromFile(name)
		return err
	})
	return meta, err
}

func (w *ExifToolWrapper) Close() error {
	w.tool.Stop()
	return nil
}