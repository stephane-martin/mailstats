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

// TODO: run exiftool in sandbox

func NewExifToolWrapper() (*ExifToolWrapper, error) {
	path, err := exec.LookPath("exiftool")
	if err != nil {
		return nil, Absent("exiftool")
	}
	t, err := exiftool.NewPool(path, runtime.NumCPU(), "-json")
	if err != nil {
		return nil, err
	}
	return &ExifToolWrapper{tool: t}, nil
}

func (w *ExifToolWrapper) ExtractFromFile(filename string, meta map[string]interface{}, flags ...string) (map[string]interface{}, error) {
	if w == nil {
		return meta, Absent("exiftool")
	}
	res, err := w.tool.ExtractFlags(filename, flags...)
	if err != nil {
		return meta, err
	}
	attributes := make([]map[string]interface{}, 0)
	err = json.Unmarshal(res, &attributes)
	if err != nil {
		return meta, err
	}
	if len(attributes) == 0 {
		return meta, nil
	}
	delete(attributes[0], "SourceFile")
	if meta == nil {
		meta = make(map[string]interface{})
	}
	for k, v := range attributes[0] {
		meta[utils.Snake(k)] = v
	}
	return meta, nil
}

func (w *ExifToolWrapper) Extract(content []byte, meta map[string]interface{}, flags ...string) (map[string]interface{}, error) {
	if w == nil {
		return meta, Absent("exiftool")
	}
	temp, err := utils.NewTempFile(content)
	if err != nil {
		return meta, err
	}
	if meta == nil {
		meta = make(map[string]interface{})
	}
	err = temp.RemoveAfter(func(name string) error {
		_, err := w.ExtractFromFile(name, meta, flags...)
		return err
	})
	return meta, err
}

func (w *ExifToolWrapper) Close() error {
	if w == nil {
		return nil
	}
	if w.tool == nil {
		return nil
	}
	w.tool.Stop()
	w.tool = nil
	return nil
}