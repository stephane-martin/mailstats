package consumers

import (
	"encoding/json"
	"github.com/go-redis/redis"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
)

func NewRedisConsumer(args arguments.RedisArgs) (*RedisConsumer, error) {
	client, err := utils.NewRedisClient(args)
	if err != nil {
		return nil, err
	}
	return &RedisConsumer{client: client, args: args}, nil
}

func (c *RedisConsumer) Consume(features *models.FeaturesMail) error {
	b, err := json.Marshal(features)
	if err != nil {
		return err
	}
	_, err = c.client.RPush(c.args.ResultsKey, b).Result()
	return err
}

func (c *RedisConsumer) Close() error {
	return c.client.Close()
}

type RedisConsumer struct {
	client *redis.Client
	args arguments.RedisArgs
}
