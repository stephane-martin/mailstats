package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
)

type KafkaArgs struct {
	Brokers []string
	Topic string
}

func (args *KafkaArgs) Verify() error {
	v := verifier.New()
	for _, broker := range args.Brokers {
		_, _, err := net.SplitHostPort(broker)
		v.That(err == nil, "invalid broker: %s", broker)
	}
	return v.GetError()
}

func (args *KafkaArgs) Populate(c *cli.Context) {
	args.Brokers = c.GlobalStringSlice("broker")
	args.Topic = c.GlobalString("topic")
}