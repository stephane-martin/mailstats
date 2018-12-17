package consumers

import (
	"github.com/Shopify/sarama"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/stephane-martin/mailstats/models"
)

type KafkaConsumer struct {
	client sarama.AsyncProducer
	logger log15.Logger
	topic string
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
	go func() {
		for range clt.Successes() {}

		for err := range clt.Errors() {
			props := err.Msg.Metadata.(map[string]string)
			logger.Warn("Failed to deliver features to Kafka", "family", props["family"], "uid", props["uid"])
		}
	}()
	return &KafkaConsumer{
		client: clt,
		logger: logger,
		topic: topic,
	}, nil
}

func (c *KafkaConsumer) Consume(features *models.FeaturesMail) error {
	b, err := features.Encode()
	if err != nil {
		return err
	}
	m := &sarama.ProducerMessage{
		Topic: c.topic,
		Key:   sarama.StringEncoder(features.Family),
		Value: sarama.ByteEncoder(b),
		Metadata: map[string]string{
			"uid": features.UID,
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

func (c *KafkaConsumer) Close() error {
	c.client.AsyncClose()
	return nil
}
