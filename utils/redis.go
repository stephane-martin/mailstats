package utils

import (
	"github.com/go-redis/redis"
	"github.com/stephane-martin/mailstats/arguments"
	"net/url"
	"strconv"
	"strings"
)





func NewRedisClient(args arguments.RedisArgs) (*redis.Client, error) {
	if args.URL == "" {
		args.URL = "redis://127.0.0.1:6379"
	}
	u, _ := url.Parse(args.URL)
	params := u.Query()
	db := strings.TrimSpace(params.Get("db"))
	if db == "" {
		db = "0"
	}
	dbnum, _ := strconv.ParseInt(db, 10, 32)
	options := &redis.Options{
		Network: "tcp",
		Addr: u.Host,
		DB: int(dbnum),
	}
	password := strings.TrimSpace(params.Get("password"))
	if password != "" {
		options.Password = password
	}

	client := redis.NewClient(options)
	_, err := client.Ping().Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}


