package forwarders

import (
	"context"
	"github.com/gojektech/heimdall/httpclient"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
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

type HTTPForwarder struct {
	client *httpclient.Client
	url    string
	logger log15.Logger
}

func NewHTTPForwarder(url string, logger log15.Logger) *HTTPForwarder {
	return &HTTPForwarder{
		client: utils.NewHTTPClient(10*time.Second, 8, 4),
		url:    url,
		logger: logger,
	}
}

func (f *HTTPForwarder) Forward(mail *models.IncomingMail) {
	r, w := io.Pipe()
	go func() {
		err := utils.JSONEncoder(w).Encode(mail)
		_ = w.CloseWithError(err)
	}()
	go func() {
		resp, err := f.client.Post(f.url, r, jsonContentTypeHeaders)
		if err != nil {
			f.logger.Warn(
				"Failed to HTTP forward incoming mail",
				"uid", ulid.ULID(mail.UID).String(),
				"error", err,
			)
		} else if resp.StatusCode > 299 {
			f.logger.Warn("HTTP forward not successful",
				"uid", ulid.ULID(mail.UID).String(),
				"code", resp.StatusCode,
			)
		}
	}()
}

func (_ *HTTPForwarder) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (f *HTTPForwarder) Close() error {
	return nil
}
