package arguments

import (
	"github.com/deckarep/golang-set"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type CollectorArgs struct {
	Type          string
	CollectorSize int
	CollectorDir  string
}

var collectorsMap = mapset.NewSetWith(
	"filesystem",
	"channel",
	"redis",
	"rabbitmq",
)

func (args *CollectorArgs) Verify() error {
	v := verifier.New()
	v.That(collectorsMap.Contains(args.Type), "Unknown collector type")
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
