package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type CollectorArgs struct {
	Type          string
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
	_, ok := collectorsMap[args.Type]
	v.That(ok, "Unknown collector type")
	return v.GetError()
}

func (args *CollectorArgs) Populate(c *cli.Context) {
	args.Type = strings.ToLower(c.GlobalString("collector"))
	if args.Type == "" {
		args.Type = "channel"
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
