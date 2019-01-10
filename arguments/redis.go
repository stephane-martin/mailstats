package arguments

import (
	"fmt"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type RedisArgs struct {
	URL string
	ResultsKey string
	CollectorKey string
}

func (args *RedisArgs) Verify() error {
	if args.URL == "" {
		return nil
	}
	v := verifier.New()
	u, err := url.Parse(args.URL)
	v.That(err == nil, "Invalid Redis connection URL")
	v.That(u.Scheme == "redis", "In redis URL, scheme must be redis")
	v.That(len(u.Host) > 0, "redis host is empty")
	_, _, err = net.SplitHostPort(u.Host)
	v.That(err == nil, fmt.Sprintf("The redis address is invalid: %s", err))
	params := u.Query()
	db := params.Get("db")
	if len(db) > 0 {
		dbnum, err := strconv.ParseInt(db, 10, 32)
		v.That(err == nil, "db paramater must be an integer")
		v.That(dbnum >= 0, "db paramater must be positive")
	}
	return v.GetError()
}

func (args *RedisArgs) Populate(c *cli.Context) {
	args.URL = strings.TrimSpace(c.GlobalString("redis-url"))
	args.ResultsKey = strings.TrimSpace(c.GlobalString("redis-results-key"))
	args.CollectorKey = strings.TrimSpace(c.GlobalString("redis-collector-key"))
	if args.CollectorKey == "" {
		args.CollectorKey = "mailstats.collector"
	}
	if args.ResultsKey == "" {
		args.ResultsKey = "mailstats.results"
	}
}

