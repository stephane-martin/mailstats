package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"net/url"
	"strings"
)

type ForwardArgs struct {
	URL string
}

func (args ForwardArgs) Parsed() (scheme, host, port, username, password string) {
	if args.URL == "" {
		return "", "", "", "", ""
	}
	u, _ := url.Parse(args.URL)
	host, port, _ = net.SplitHostPort(u.Host)
	password, _ = u.User.Password()
	return strings.ToLower(strings.TrimSpace(u.Scheme)),
		strings.TrimSpace(host),
		strings.TrimSpace(port),
		strings.TrimSpace(u.User.Username()),
		strings.TrimSpace(password)
}

func (args *ForwardArgs) Verify() error {
	if args.URL == "" {
		return nil
	}
	u, err := url.Parse(args.URL)
	v := verifier.New()
	v.That(err == nil, "Invalid SMTP forward URL")
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	v.That(scheme == "smtp" || scheme == "smtps", "Forward URL scheme is not smtp")
	v.That(len(u.Host) > 0, "Forward host is empty")
	h, p, err := net.SplitHostPort(u.Host)
	v.That(err == nil, "Forward host must be host:port")
	v.That(len(h) > 0, "Forward host is empty")
	v.That(len(p) > 0, "Forward port is empty")
	return v.GetError()
}

func (args *ForwardArgs) Populate(c *cli.Context) {
	if args == nil {
		args = new(ForwardArgs)
	}
	args.URL = strings.TrimSpace(c.GlobalString("forward"))
}
