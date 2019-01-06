package consumers

import (
	"context"
	"encoding/json"
	"github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/models"
	"net/http"
	"time"
)

type ElasticsearchConsumer struct {
	client *elastic.Client
	processor *elastic.BulkProcessor
	indexName string
}

func NewElasticsearchConsumer(urls []string, indexName string, logger log15.Logger) (*ElasticsearchConsumer, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	client, err := elastic.NewClient(
		elastic.SetErrorLog(&logging.PrintfLogger{Logger: logger}),
		elastic.SetHealthcheck(true),
		elastic.SetSniff(false),
		elastic.SetRetrier(
			elastic.NewBackoffRetrier(
				elastic.NewExponentialBackoff(10 * time.Second, 120 * time.Second),
			),
		),
		elastic.SetHealthcheckInterval(elastic.DefaultHealthcheckInterval),
		elastic.SetHealthcheck(true),
		elastic.SetGzip(true),
		elastic.SetHealthcheckTimeout(10*time.Second),
		elastic.SetHealthcheckTimeoutStartup(15 * time.Second),
		elastic.SetHttpClient(httpClient),
		elastic.SetURL(urls...),
	)
	if err != nil {
		return nil, err
	}

	return &ElasticsearchConsumer{
		client:    client,
		indexName: indexName,
	}, nil
}

func (c *ElasticsearchConsumer) Name() string {
	return "ElasticsearchConsumer"
}

func (c *ElasticsearchConsumer) Prestart() error {
	processor, err := c.client.BulkProcessor().
		Name("mailstats_bulk_processor").
		BulkActions(-1).
		BulkSize(-1).
		Stats(false).
		Workers(1).
		FlushInterval(5 * time.Second).Do(context.Background())

	if err != nil {
		return err
	}
	c.processor = processor
	return nil
}

func (c *ElasticsearchConsumer) Consume(features *models.FeaturesMail) error {
	b, err := features.Encode(false)
	if err != nil {
		return err
	}
	c.processor.Add(
		elastic.NewBulkIndexRequest().Index(c.indexName).Type(c.indexName).Doc(json.RawMessage(b)),
	)
	return nil
}

func (c *ElasticsearchConsumer) Close() (err error) {
	if c.processor != nil {
		err = c.processor.Close()
		c.processor = nil
	}
	if c.client != nil {
		c.client.Stop()
		c.client = nil
	}
	return err
}
