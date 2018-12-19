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
		return nil, Absent("exiftool", err)
	}
	t, err := exiftool.NewPool(path, runtime.NumCPU(), "-json")
	if err != nil {
		return nil, err
	}
	return &ExifToolWrapper{tool: t}, nil
}

func (w *ExifToolWrapper) ExtractFromFile(filename string, flags ...string) (map[string]interface{}, error) {
	res, err := w.tool.ExtractFlags(filename, flags...)
	if err != nil {
		return nil, err
	}
	results := make(map[string]interface{})
	attributes := make([]map[string]interface{}, 0)
	err = json.Unmarshal(res, &attributes)
	if err != nil {
		return nil, err
	}
	if len(attributes) == 0 {
		return results, nil
	}
	delete(attributes[0], "SourceFile")
	for k, v := range attributes[0] {
		results[utils.Snake(k)] = v
	}
	return results, nil
}

func (w *ExifToolWrapper) Extract(content []byte, flags ...string) (meta map[string]interface{}, err error) {
	temp, err := utils.NewTempFile(content)
	if err != nil {
		return nil, err
	}
	_ = temp.RemoveAfter(func(name string) error {
		meta, err = w.ExtractFromFile(name, flags...)
		return err
	})
	return meta, err
}

func (w *ExifToolWrapper) Close() error {
	w.tool.Stop()
	return nil
}