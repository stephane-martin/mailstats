package consumers

import (
	"github.com/go-redis/redis"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
)

func NewRedisConsumer(args arguments.RedisArgs, redis utils.RedisConn) (*RedisConsumer, error) {
	return &RedisConsumer{client: redis.Client(), args: args}, nil
}

func (c *RedisConsumer) Name() string {
	return "RedisConsumer"
}

func (c *RedisConsumer) Consume(features *models.FeaturesMail) error {
	b, err := utils.JSONMarshal(features)
	if err != nil {
		return err
	}
	_, err = c.client.RPush(c.args.ResultsKey, b).Result()
	return err
}

type RedisConsumer struct {
	client *redis.Client
	args arguments.RedisArgs
}

