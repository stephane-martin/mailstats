package main

import (
	"runtime"
	"strings"

	"github.com/awnumar/memguard"
	"github.com/urfave/cli"
)

type Args struct {
	SMTP              SMTPArgs
	Milter            MilterArgs
	HTTP              HTTPArgs
	Redis             RedisArgs
	Consumer          ConsumerArgs
	Logging           LoggingArgs
	Forward           ForwardArgs
	Collector         string
	CollectorSize     int
	CollectorDir      string
	RedisCollectorKey string
	Secret            *memguard.LockedBuffer
	NbParsers         int
}

func GetArgs(c *cli.Context) (*Args, error) {
	args := new(Args)

	args.SMTP.Populate(c)
	err := args.SMTP.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Milter.Populate(c)
	err = args.Milter.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.HTTP.Populate(c)
	err = args.HTTP.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Redis.Populate(c)
	err = args.Redis.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Consumer.Populate(c)
	err = args.Consumer.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Logging.Populate(c)
	err = args.Logging.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Forward.Populate(c)
	err = args.Logging.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Collector = strings.ToLower(c.GlobalString("collector"))
	if args.Collector == "" {
		args.Collector = "channel"
	}
	if args.Collector != "filesystem" && args.Collector != "channel" && args.Collector != "redis" {
		return nil, cli.NewExitError("Unknown collector type", 1)
	}

	args.CollectorDir = c.GlobalString("collector-dir")
	if args.CollectorDir == "" {
		args.CollectorDir = "/var/lib/mailstats"
	}

	args.CollectorSize = c.GlobalInt("collector-size")
	if args.CollectorSize <= 0 {
		args.CollectorSize = 10000
	}

	args.RedisCollectorKey = strings.TrimSpace(c.GlobalString("redis-collector-key"))
	if args.RedisCollectorKey == "" {
		args.RedisCollectorKey = "mailstats.collector"
	}

	sec := strings.TrimSpace(c.GlobalString("secret"))
	if len(sec) > 0 {
		secret, err := memguard.NewImmutableFromBytes([]byte(sec))
		if err != nil {
			return nil, cli.NewExitError("memguard failed", 1)
		}
		args.Secret = secret
	}
	args.NbParsers = c.GlobalInt("nbparsers")
	if args.NbParsers == -1 {
		args.NbParsers = runtime.NumCPU()
	}

	return args, nil
}
