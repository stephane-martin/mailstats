package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"strings"
)

type SMTPArgs struct {
	ListenAddr     string
	ListenPort     int
	MaxMessageSize int
	MaxIdle        int
	Inetd          bool
}

func (args SMTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The listen address is empty")
	v.That(args.MaxMessageSize >= 0, "The message size must be positive")
	v.That(args.MaxIdle >= 0, "The idle time must be positive")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The listen address is invalid")
	return v.GetError()
}

func (args *SMTPArgs) Populate(c *cli.Context) *SMTPArgs {
	if args == nil {
		args = new(SMTPArgs)
	}
	args.ListenPort = c.Int("lport")
	if args.ListenPort == 0 {
		args.ListenPort = 3333
	}
	args.ListenAddr = strings.TrimSpace(c.String("laddr"))
	if args.ListenAddr == "" {
		args.ListenAddr = "127.0.0.1"
	}
	args.MaxMessageSize = c.Int("max-size")
	args.MaxIdle = c.Int("max-idle")
	args.Inetd = c.GlobalBool("inetd")
	return args
}

