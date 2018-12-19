package arguments

import (
	"runtime"
	"strings"

	"github.com/awnumar/memguard"
	"github.com/urfave/cli"
)

type Args struct {
	SMTP      SMTPArgs
	Milter    MilterArgs
	HTTP      HTTPArgs
	Redis     RedisArgs
	Consumer  ConsumerArgs
	Logging   LoggingArgs
	Forward   ForwardArgs
	Collector CollectorArgs
	Rabbit    RabbitArgs
	Kafka     KafkaArgs
	GeoIP     GeoIPArgs
	Secret    *memguard.LockedBuffer
	NbParsers int
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
	}

	for _, i := range toInit {
		i.Populate(c)
		err := i.Verify()
		if err != nil {
			return nil, cli.NewExitError(err.Error(), 1)
		}
	}

	sec := strings.TrimSpace(c.GlobalString("secret"))
	if len(sec) > 0 {
		secret, err := memguard.NewImmutableFromBytes([]byte(sec))
		if err != nil {
			return nil, cli.NewExitError("memguard failed", 1)
		}
		args.Secret = secret
	}
	args.NbParsers = c.GlobalInt("nbparsers")
	if args.NbParsers == -1 {
		args.NbParsers = runtime.NumCPU()
	}

	return args, nil
}

