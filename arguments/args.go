package arguments

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/awnumar/memguard"
	"github.com/urfave/cli"
)

type Args struct {
	SMTP          SMTPArgs
	Milter        MilterArgs
	HTTP          HTTPArgs
	Redis         RedisArgs
	Consumer      ConsumerArgs
	Logging       LoggingArgs
	Forward       ForwardArgs
	Collector     CollectorArgs
	Rabbit        RabbitArgs
	Kafka         KafkaArgs
	GeoIP         GeoIPArgs
	Elasticsearch ElasticsearchArgs
	Secret        *memguard.LockedBuffer `json:"-"`
	NbParsers     int
	NoDKIM        bool
}

type argsI interface {
	Populate(c *cli.Context)
	Verify() error
}

func GetArgs(c *cli.Context) (*Args, error) {
	args := new(Args)

	toInit := []argsI{
		&args.SMTP,
		&args.Milter,
		&args.HTTP,
		&args.Redis,
		&args.Consumer,
		&args.Logging,
		&args.Forward,
		&args.Collector,
		&args.Rabbit,
		&args.Kafka,
		&args.GeoIP,
		&args.Elasticsearch,
	}

	for _, i := range toInit {
		i.Populate(c)
		err := i.Verify()
		if err != nil {
			return nil, err
		}
	}

	sec := strings.TrimSpace(c.GlobalString("secret"))
	if len(sec) > 0 {
		secret, err := memguard.NewImmutableFromBytes([]byte(sec))
		if err != nil {
			return nil, fmt.Errorf("memguard failure: %s", err)
		}
		args.Secret = secret
	}
	args.NbParsers = c.GlobalInt("nbparsers")
	if args.NbParsers == -1 {
		args.NbParsers = runtime.NumCPU()
	}
	args.NoDKIM = c.GlobalBool("no-dkim")

	return args, nil
}

func (args *Args) RedisRequired() bool {
	return args.Consumer.GetType() == Redis || args.Collector.Type == "redis"
}
