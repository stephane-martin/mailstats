package consumers

import (
	"encoding/json"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"io"
	"os"
	"sync"
)

var printLock sync.Mutex

type Writer struct {
	io.WriteCloser
}

func (w Writer) Consume(features *models.FeaturesMail) (err error) {
	printLock.Lock()
	err = json.NewEncoder(w.WriteCloser).Encode(features)
	printLock.Unlock()
	return err
}


var StdoutConsumer Consumer = Writer{WriteCloser: os.Stdout}
var StderrConsumer Consumer = Writer{WriteCloser: os.Stderr}

func NewFileConsumer(args arguments.ConsumerArgs) (Consumer, error) {
	f, err := os.OpenFile(args.OutFile, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0664)
	if err != nil {
		return nil, err
	}
	return Writer{WriteCloser: f}, nil
}


