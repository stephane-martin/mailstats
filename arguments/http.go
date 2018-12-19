package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"strings"
)

type HTTPArgs struct {
	ListenAddr string
	ListenPort int
}

func (args *HTTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The HTTP listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The HTTP listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The HTTP listen address is invalid")
	return v.GetError()
}

func (args *HTTPArgs) Populate(c *cli.Context) {
	args.ListenPort = c.GlobalInt("http-port")
	args.ListenAddr = strings.TrimSpace(c.GlobalString("http-addr"))
}
