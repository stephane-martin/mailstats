package collectors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"strings"
	"time"
)

type RedisCollector struct {
	logger log15.Logger
	client *redis.Client
	key    string
	curkey string
}

func NewRedisCollector(args arguments.RedisArgs, logger log15.Logger) (*RedisCollector, error) {
	client, err := utils.NewRedisClient(args)
	if err != nil {
		return nil, err
	}
	return &RedisCollector{
		logger: logger,
		client: client,
		key:    args.CollectorKey,
		curkey: args.CollectorKey + "_pending",
	}, nil
}

func (c *RedisCollector) Name() string {
	return "RedisCollector"
}

func (c *RedisCollector) rePush() error {
	now := time.Now()
	return c.client.Watch(func(tx *redis.Tx) error {
		keys, err := tx.HKeys(c.curkey).Result()
		if err != nil {
			return err
		}
		uids := make([]string, 0)
		for _, key := range keys {
			uid := ulid.MustParse(key)
			if now.Sub(ulid.Time(uid.Time())) >= time.Minute {
				uids = append(uids, uid.String())
			}
		}
		results, err := tx.HMGet(c.curkey, uids...).Result()
		if err != nil {
			return err
		}
		msgs := make([]*models.IncomingMail, 0, len(results))
		for _, result := range results {
			var m models.IncomingMail
			_, err := m.UnmarshalMsg(result.([]byte))
			if err == nil {
				msgs = append(msgs, &m)
			}
		}
		if len(msgs) == 0 {
			return nil
		}
		_, err = tx.Pipelined(func(p redis.Pipeliner) error {
			for _, msg := range msgs {
				return rpush(p, c.key, msg)
			}
			return nil
		})
		return err
	}, c.curkey)
}

func (c *RedisCollector) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
			var err error
		Retry:
			for {
				err = c.rePush()
				if err == nil || err != redis.TxFailedErr {
					break Retry
				}
			}
			if err != nil {
				return err
			}
		}
	}
}

type pusher interface {
	RPush(key string, values ...interface{}) *redis.IntCmd
}

func rpush(p pusher, key string, m *models.IncomingMail) error {
	m.UID = utils.NewULID()
	var buffer bytes.Buffer
	err := utils.Compress(&buffer, m)
	if err != nil {
		return err
	}
	err = p.RPush(key, buffer.Bytes()).Err()
	if err == nil {
		metrics.M().CollectorSize.Inc()
	}
	return err
}

func (c *RedisCollector) Push(stop <-chan struct{}, m *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(m.MailFrom, m.Family).Inc()
	for _, r := range m.RcptTo {
		metrics.M().MailTo.WithLabelValues(r, m.Family).Inc()
	}
	return rpush(c.client, c.key, m)
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
	if len(res) != 2 {
		return nil, fmt.Errorf("wrong number of returned variables by BLPOP: %d", len(res))
	}
	if len(res[1]) == 0 {
		return nil, errors.New("empty string returned by BLPOP")
	}
	metrics.M().CollectorSize.Dec()
	//c.logger.Debug("BLPOP result", "length", len(res[1]), "key", res[0])
	m := new(models.IncomingMail)
	err = utils.Decompress(strings.NewReader(res[1]), m)
	if err != nil {
		return nil, fmt.Errorf("messagepack unmarshal error: %s", err)
	}
	b, _ := m.MarshalMsg(nil)
	err = c.client.HSet(c.curkey, ulid.ULID(m.UID).String(), b).Err()
	if err != nil {
		c.logger.Warn("Error while storing pending message in Redis", "error", err)
	}

	return m, nil
}

func (c *RedisCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *RedisCollector) Close() error {
	return c.client.Close()
}

func (c *RedisCollector) ACK(uid ulid.ULID) {
	err := c.client.HDel(c.curkey, uid.String()).Err()
	if err != nil {
		c.logger.Warn("ACK error with Redis collector", "error", err)
	}
}
