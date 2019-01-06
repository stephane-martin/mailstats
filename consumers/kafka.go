package consumers

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/stephane-martin/mailstats/models"
)

type KafkaConsumer struct {
	client sarama.AsyncProducer
	logger log15.Logger
	topic  string
}

func NewKafkaConsumer(brokers []string, topic string, logger log15.Logger) (*KafkaConsumer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("Kafka brokers are not specified")
	}
	conf := sarama.NewConfig()
	conf.Producer.Compression = sarama.CompressionLZ4
	conf.Producer.CompressionLevel = 0
	conf.Producer.MaxMessageBytes = 10000000
	conf.Producer.Return.Errors = true
	conf.Producer.Return.Successes = true
	conf.Producer.RequiredAcks = sarama.WaitForLocal
	conf.ClientID = "mailstats"
	conf.Version = sarama.V1_0_0_0
	clt, err := sarama.NewAsyncProducer(brokers, conf)
	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		client: clt,
		logger: logger,
		topic:  topic,
	}, nil
}

func (c *KafkaConsumer) Start(ctx context.Context) error {
	succs := c.client.Successes()
	errs := c.client.Errors()
	done := ctx.Done()
	for {
		if succs == nil && errs == nil && done == nil {
			return nil
		}
		select {
		case _, ok := <-succs:
			if !ok {
				succs = nil
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
			} else {
				props := err.Msg.Metadata.(map[string]string)
				c.logger.Warn("Failed to deliver features to Kafka", "family", props["family"], "uid", props["uid"])
			}
		case <-done:
			c.client.AsyncClose()
			done = nil
		}
	}
}

func (c *KafkaConsumer) Consume(features *models.FeaturesMail) error {
	b, err := features.Encode(false)
	if err != nil {
		return err
	}
	m := &sarama.ProducerMessage{
		Topic: c.topic,
		Key:   sarama.StringEncoder(features.Family),
		Value: sarama.ByteEncoder(b),
		Metadata: map[string]string{
			"uid":    features.UID,
			"family": features.Family,
		},
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("Content-Type"),
				Value: []byte("application/json"),
			},
		},
	}
	c.client.Input() <- m
	return nil
}

func (c *KafkaConsumer) Name() string {
	return "KafkaConsumer"
}
