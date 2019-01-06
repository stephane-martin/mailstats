package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type LoggingArgs struct {
	LogLevel string
	Syslog bool
}

func (args *LoggingArgs) Populate(c *cli.Context) {
	args.LogLevel = strings.ToLower(strings.TrimSpace(c.GlobalString("loglevel")))
	args.Syslog = c.GlobalBool("syslog")
}

func (args *LoggingArgs) Verify() error {
	v := verifier.New()
	v.That(len(args.LogLevel) > 0, "loglevel is empty")
	return v.GetError()
}

