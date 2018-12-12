package arguments

import (
	"fmt"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net/url"
	"strings"
)

type RabbitArgs struct {
	URI                 string
	CollectorQueue      string
	CollectorExchange   string
	ResultsExchange     string
	ResultsExchangeType string
	ResultsRoutingKey   string
}

func (args RabbitArgs) Verify() error {
	v := verifier.New()
	_, err := url.Parse(args.URI)
	v.That(err == nil, fmt.Sprintf("Invalid RabbitMQ URI: %s", err))
	return v.GetError()
}

func (args *RabbitArgs) Populate(c *cli.Context) *RabbitArgs {
	if args == nil {
		args = new(RabbitArgs)
	}
	args.URI = strings.TrimSpace(c.GlobalString("rabbitmq-uri"))
	args.CollectorExchange = strings.TrimSpace(c.GlobalString("rabbitmq-collector-exchange"))
	args.CollectorQueue = strings.TrimSpace(c.GlobalString("rabbitmq-collector-queue"))
	args.ResultsExchange = strings.TrimSpace(c.GlobalString("rabbitmq-results-exchange"))
	args.ResultsExchangeType = strings.TrimSpace(c.GlobalString("rabbitmq-results-exchange-type"))
	args.ResultsRoutingKey = strings.TrimSpace(c.GlobalString("rabbitmq-results-routing-key"))
	return args
}
