package collectors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/inconshreveable/log15"
	"github.com/pierrec/lz4"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/tinylib/msgp/msgp"
	"strings"
	"time"
)

type RedisCollector struct {
	logger log15.Logger
	client *redis.Client
	key    string
}

func NewRedisCollector(args *arguments.Args, logger log15.Logger) (*RedisCollector, error) {
	client, err := utils.NewRedisClient(args.Redis)
	if err != nil {
		return nil, err
	}
	return &RedisCollector{logger: logger, client: client, key: args.RedisCollectorKey}, nil
}

func (c *RedisCollector) Push(stop <-chan struct{}, info *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(info.MailFrom).Inc()
	for _, r := range info.RcptTo {
		metrics.M().MailTo.WithLabelValues(r).Inc()
	}
	var buffer bytes.Buffer
	w := lz4.NewWriter(&buffer)
	w.Header = lz4.Header{
		CompressionLevel: 0,
	}
	err := utils.Autoclose(w, func() error {
		return msgp.Encode(w, info)
	})
	_, err = c.client.RPush(c.key, buffer.Bytes()).Result()
	if err == nil {
		metrics.M().CollectorSize.Inc()
	}
	return err
}

func (c *RedisCollector) PushCtx(ctx context.Context, info *models.IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *RedisCollector) Pull(stop <-chan struct{}) (*models.IncomingMail, error) {
	var res []string
	var err error
	gotIt := make(chan struct{})
	go func() {
		for {
			res, err = c.client.BLPop(time.Second, c.key).Result()
			select {
			case <-stop:
				if err == nil && len(res) > 0 {
					// we pulled some work from redis, but the client is already gone
					// so we need to push back work to redis
					_, err := c.client.RPush(c.key, res[1]).Result()
					if err != nil {
						c.logger.Error("Error pushing back work to redis", "error", err)
					}
				}
				return
			default:
				if (err != nil && err != redis.Nil) || len(res) > 0 {
					close(gotIt)
					return
				}
			}
		}
	}()
	select {
	case <-stop:
		return nil, context.Canceled
	case <-gotIt:
	}
	if err != nil {
		return nil, fmt.Errorf("BLPOP error: %s", err)
	}
	metrics.M().CollectorSize.Dec()
	if len(res) != 2 {
		return nil, fmt.Errorf("wrong number of returned variables by BLPOP: %d", len(res))
	}
	if len(res[1]) == 0 {
		return nil, errors.New("empty string returned by BLPOP")
	}
	c.logger.Debug("BLPOP result", "length", len(res[1]), "key", res[0])
	raw := strings.NewReader(res[1])
	lz4Reader := lz4.NewReader(raw)
	var mail models.IncomingMail
	err = msgp.Decode(lz4Reader, &mail)
	if err != nil {
		return nil, fmt.Errorf("messagepack unmarshal error: %s", err)
	}
	return &mail, nil
}

func (c *RedisCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *RedisCollector) Close() error {
	return c.client.Close()
}
