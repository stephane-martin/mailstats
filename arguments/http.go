package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"strconv"
	"strings"
)

type HTTPArgs struct {
	HostPortAPI    string
	HostPortMaster string
	ListenAddrAPI  string
	ListenPortAPI  int
	ListenAddrMaster  string
	ListenPortMaster  int

}

func (args *HTTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.HostPortAPI != args.HostPortMaster, "API and master listen addresses must be different")

	host, portStr, err := net.SplitHostPort(args.HostPortAPI)
	v.That(err == nil, "Invalid HTTP listen address")
	port, err := strconv.ParseInt(portStr, 10, 64)
	v.That(err == nil, "Can't parse the port from HTTP listen address")
	args.ListenAddrAPI = host
	args.ListenPortAPI = int(port)
	v.That(args.ListenPortAPI > 0, "The HTTP listen port must be positive")
	v.That(len(args.ListenAddrAPI) > 0, "The HTTP listen address is empty")
	p := net.ParseIP(args.ListenAddrAPI)
	v.That(p != nil, "The HTTP listen address is invalid")

	host, portStr, err = net.SplitHostPort(args.HostPortMaster)
	v.That(err == nil, "Invalid master listen address")
	port, err = strconv.ParseInt(portStr, 10, 64)
	v.That(err == nil, "Can't parse the port from master listen address")
	args.ListenAddrMaster = host
	args.ListenPortMaster = int(port)
	v.That(args.ListenPortMaster > 0, "The master listen port must be positive")
	v.That(len(args.ListenAddrMaster) > 0, "The master listen address is empty")
	p = net.ParseIP(args.ListenAddrMaster)
	v.That(p != nil, "The master listen address is invalid")

	return v.GetError()
}

func (args *HTTPArgs) Populate(c *cli.Context) {
	args.HostPortAPI = strings.TrimSpace(c.GlobalString("api-listen-addr"))
	args.HostPortMaster = strings.TrimSpace(c.GlobalString("master-listen-addr"))
}
