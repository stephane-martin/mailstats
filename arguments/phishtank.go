package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type PhishtankArgs struct {
	ApplicationKey string
	CacheDir string
	Active bool
}

func (args *PhishtankArgs) Verify() error {
	if !args.Active {
		return nil
	}
	v := verifier.New()
	v.That(args.ApplicationKey != "", "Provide a Phishtank application key (register on website)")
	return v.GetError()
}

func (args *PhishtankArgs) Populate(c *cli.Context) {
	args.Active = c.GlobalBool("phishtank")
	args.CacheDir = strings.TrimSpace(c.GlobalString("cache-dir"))
	if args.CacheDir == "" {
		args.CacheDir = "/var/lib/mailstats"
	}
	args.ApplicationKey = strings.TrimSpace(c.GlobalString("phishtank-appkey"))
}