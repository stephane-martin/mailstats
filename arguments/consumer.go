package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net/url"
	"strings"
)

type ConsumerType int

const (
	Stdout ConsumerType = iota
	Stderr
	File
	Redis
	HTTP
	Rabbit
)

var ConsumerTypes = map[string]ConsumerType{
	"stdout": Stdout,
	"stderr": Stderr,
	"file": File,
	"redis": Redis,
	"http": HTTP,
	"rabbitmq": Rabbit,
}

type ConsumerArgs struct {
	Type string
	OutFile string
	OutURL string
}

func (args ConsumerArgs) GetURL() string {
	u, _ := url.Parse(args.OutURL)
	return u.String()
}

func (args ConsumerArgs) GetType() ConsumerType {
	return ConsumerTypes[args.Type]
}

func (args ConsumerArgs) Verify() error {
	v := verifier.New()
	_, ok := ConsumerTypes[args.Type]
	v.That(ok, "consumer type unknown")
	v.That(len(args.OutFile) > 0, "the output filename is empty")
	_, err := url.Parse(args.OutURL)
	v.That(err == nil, "invalid out URL: %s", err)
	return v.GetError()
}

func (args *ConsumerArgs) Populate(c *cli.Context) *ConsumerArgs {
	if args == nil {
		args = new(ConsumerArgs)
	}
	args.Type = strings.ToLower(strings.TrimSpace(c.GlobalString("out")))
	args.OutFile = strings.ToLower(strings.TrimSpace(c.GlobalString("outfile")))
	args.OutURL = strings.ToLower(strings.TrimSpace(c.GlobalString("outurl")))
	return args
}
