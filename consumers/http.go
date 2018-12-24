package consumers

import (
	"github.com/gojektech/heimdall/httpclient"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"net/http"
	"time"
)

var jsonContentTypeHeaders http.Header

func init() {
	jsonContentTypeHeaders = make(map[string][]string)
	jsonContentTypeHeaders.Set("Content-Type", "application/json")
}

type HTTPConsumer struct {
	client *httpclient.Client
	url    string
}

func NewHTTPConsumer(args arguments.ConsumerArgs) (Consumer, error) {
	return &HTTPConsumer{
		client: utils.NewHTTPClient(10*time.Second, 8, 4),
		url:    args.GetURL(),
	}, nil
}

func (c *HTTPConsumer) Consume(features *models.FeaturesMail) error {
	r, w := io.Pipe()
	go func() {
		err := utils.JSONEncoder(w).Encode(features)
		_ = w.CloseWithError(err)
	}()
	_, err := c.client.Post(c.url, r, jsonContentTypeHeaders)
	return err
}

func (c *HTTPConsumer) Close() error {
	return nil
}
