package consumers

import (
	"encoding/json"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"net/http"
	"time"
)

type HTTPConsumer struct {
	client *http.Client
	url string
}

func NewHTTPConsumer(args arguments.ConsumerArgs) (Consumer, error) {
	return &HTTPConsumer{
		client: utils.NewHTTPClient(10*time.Second),
		url: args.GetURL(),
	}, nil
}

func (c *HTTPConsumer) Consume(features *models.FeaturesMail) error {
	r, w := io.Pipe()
	go func() {
		err := json.NewEncoder(w).Encode(features)
		_ = w.CloseWithError(err)
	}()
	_, err := c.client.Post(c.url, "application/json", r)
	return err
}

func (c *HTTPConsumer) Close() error {
	return nil
}
