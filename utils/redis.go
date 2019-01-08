package utils

import (
	"github.com/go-redis/redis"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"go.uber.org/fx"
	"net/url"
	"strconv"
	"strings"
)

type RedisResult struct {
	fx.Out
	Collector RedisConn `name:"collector"`
	Consumer  RedisConn `name:"consumer"`
}

type RedisConn interface {
	Service
	Prestartable
	Closeable
	Client() *redis.Client
}

var RedisService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, logger log15.Logger) (RedisResult, error) {
	var collector, consumer RedisConn
	var err error

	if args.Collector.Type == "redis" {
		collector, err = NewRedisClient(args.Redis.URL)
		if err != nil {
			return RedisResult{}, err
		}
	}
	if args.Consumer.GetType() == arguments.Redis {
		consumer, err = NewRedisClient(args.Redis.URL)
		if err != nil {
			return RedisResult{}, err
		}
	}

	Append(lc, collector, logger)
	Append(lc, consumer, logger)
	return RedisResult{
		Collector: collector,
		Consumer: consumer,
	}, nil
})

type redisConnImpl struct {
	client *redis.Client
}

func (c *redisConnImpl) Name() string {
	return "Redis"
}

func (c *redisConnImpl) Prestart() error {
	_, err := c.client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func (c *redisConnImpl) Close() error {
	return c.client.Close()
}

func (c *redisConnImpl) Client() *redis.Client {
	return c.client
}

func NewRedisClient(uri string) (RedisConn, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		uri = "redis://127.0.0.1:6379"
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	params := u.Query()
	db := strings.TrimSpace(params.Get("db"))
	if db == "" {
		db = "0"
	}
	dbnum, _ := strconv.ParseInt(db, 10, 32)
	options := &redis.Options{
		Network: "tcp",
		Addr:    u.Host,
		DB:      int(dbnum),
	}
	password := strings.TrimSpace(params.Get("password"))
	if password != "" {
		options.Password = password
	}

	client := redis.NewClient(options)
	return &redisConnImpl{client: client}, nil
}
