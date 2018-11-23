package main

import (
	"github.com/go-redis/redis"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"strings"
)

type RedisArgs struct {
	Addr string
	ResultsDB int
	ResultsKey string
}

func (args RedisArgs) Verify() error {
	v := verifier.New()
	v.That(args.ResultsDB >= 0, "Redis database must be a positive integer")
	_, _, err := net.SplitHostPort(args.Addr)
	v.That(err == nil, "The redis address is invalid")
	v.That(len(args.ResultsKey) > 0, "The redis key for results is empty")
	return v.GetError()
}

func (args *RedisArgs) Populate(c *cli.Context) *RedisArgs {
	if args == nil {
		args = new(RedisArgs)
	}
	args.Addr = strings.TrimSpace(c.GlobalString("redis-addr"))
	args.ResultsKey = strings.TrimSpace(c.GlobalString("redis-results-key"))
	args.ResultsDB = c.GlobalInt("redis-results")
	return args
}

type RedisConsumer struct {
	client *redis.Client
	args RedisArgs
}

func NewRedisConsumer(args RedisArgs) (*RedisConsumer, error) {
	client := redis.NewClient(&redis.Options{
		Addr: args.Addr,
		DB: args.ResultsDB,
	})
	_, err := client.Ping().Result()
	if err != nil {
		return nil, err
	}
	return &RedisConsumer{client: client, args: args}, nil
}

func (c *RedisConsumer) Consume(infos string) error {
	_, err := c.client.RPush(c.args.ResultsKey, infos).Result()
	return err
}

func (c *RedisConsumer) Close() error {
	return c.client.Close()
}
