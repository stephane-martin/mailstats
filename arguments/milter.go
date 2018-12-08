package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"strings"
)

type MilterArgs struct {
	ListenAddr string
	ListenPort int
	Inetd      bool
}

func (args MilterArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The listen address is invalid")
	return v.GetError()
}

func (args *MilterArgs) Populate(c *cli.Context) *MilterArgs {
	if args == nil {
		args = new(MilterArgs)
	}
	args.ListenPort = c.Int("lport")
	if args.ListenPort == 0 {
		args.ListenPort = 3333
	}
	args.ListenAddr = strings.TrimSpace(c.String("laddr"))
	if args.ListenAddr == "" {
		args.ListenAddr = "127.0.0.1"
	}
	args.Inetd = c.GlobalBool("inetd")
	return args
}
