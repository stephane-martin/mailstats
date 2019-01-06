package extractors

import (
	"encoding/json"
	"github.com/inconshreveable/log15"
	"github.com/mostlygeek/go-exiftool"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
	"os/exec"
	"runtime"
)

var ExifToolBinary = "exiftool"

type ExifTool interface {
	utils.Service
	utils.Prestartable
	utils.Closeable
	Extract(content []byte, meta map[string]interface{}, flags ...string) (map[string]interface{}, error)
	ExtractFromFile(filename string, meta map[string]interface{}, flags ...string) (map[string]interface{}, error)
}

type ExifToolImpl struct {
	path string
	tool *exiftool.Pool
	logger log15.Logger
}

// TODO: run exiftool in sandbox

func NewExifTool(logger log15.Logger) ExifTool {
	if logger == nil {
		logger = log15.New()
		logger.SetHandler(log15.DiscardHandler())
	}
	path, err := exec.LookPath(ExifToolBinary)
	if err != nil {
		logger.Info("exiftool not found", "error", err)
		return nil
	}
	return &ExifToolImpl{
		path: path,
		logger: logger,
	}
}

var ExifToolService = fx.Provide(func(lc fx.Lifecycle, logger log15.Logger) ExifTool {
	t := NewExifTool(logger)
	if t != nil {
		utils.Append(lc, t, logger)
	}
	return t
})

func (w *ExifToolImpl) Name() string { return "ExifTool" }

func (w *ExifToolImpl) Prestart() error {
	t, err := exiftool.NewPool(w.path, runtime.NumCPU(), "-json")
	if err != nil {
		w.logger.Info("failed to make exiftool pool", "error", err)
		return err
	}
	w.tool = t
	return nil
}

func (w *ExifToolImpl) ExtractFromFile(filename string, meta map[string]interface{}, flags ...string) (map[string]interface{}, error) {
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

func (w *ExifToolImpl) Extract(content []byte, meta map[string]interface{}, flags ...string) (map[string]interface{}, error) {
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

func (w *ExifToolImpl) Close() error {
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


