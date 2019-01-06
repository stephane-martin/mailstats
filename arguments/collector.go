package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type CollectorArgs struct {
	Collector     string
	CollectorSize int
	CollectorDir  string
}

var collectorsMap = map[string]struct{}{
	"filesystem": {},
	"channel":    {},
	"redis":      {},
	"rabbitmq":   {},
}

func (args *CollectorArgs) Verify() error {
	v := verifier.New()
	_, ok := collectorsMap[args.Collector]
	v.That(ok, "Unknown collector type")
	return v.GetError()
}

func (args *CollectorArgs) Populate(c *cli.Context) {
	args.Collector = strings.ToLower(c.GlobalString("collector"))
	if args.Collector == "" {
		args.Collector = "channel"
	}

	args.CollectorDir = c.GlobalString("collector-dir")
	if args.CollectorDir == "" {
		args.CollectorDir = "/var/lib/mailstats"
	}

	args.CollectorSize = c.GlobalInt("collector-size")
	if args.CollectorSize <= 0 {
		args.CollectorSize = 10000
	}
}
