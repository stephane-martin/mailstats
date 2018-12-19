package arguments

import (
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"strings"
)

type GeoIPArgs struct {
	Enabled bool
	DatabasePath string
}

func (args *GeoIPArgs) Populate(c *cli.Context) {
	args.Enabled = c.GlobalBool("geoip")
	args.DatabasePath = strings.TrimSpace(c.GlobalString("geoip-database-path"))
}

func (args GeoIPArgs) Verify() error {
	v := verifier.New()
	if args.Enabled {
		v.That(args.DatabasePath != "", "Specify GeoIP database")
		if args.DatabasePath != "" {
			r, err := geoip2.Open(args.DatabasePath)
			v.That(err == nil, fmt.Sprintf("Error loading database path: %s", err))
			if err != nil {
				_ = r.Close()
			}
		}
	}
	return v.GetError()
}
