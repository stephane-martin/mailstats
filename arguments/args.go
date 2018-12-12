package arguments

import (
	"github.com/storozhukBM/verifier"
	"net"
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
	Secret    *memguard.LockedBuffer
	NbParsers int
}

func GetArgs(c *cli.Context) (*Args, error) {
	args := new(Args)

	args.SMTP.Populate(c)
	err := args.SMTP.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Milter.Populate(c)
	err = args.Milter.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.HTTP.Populate(c)
	err = args.HTTP.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Redis.Populate(c)
	err = args.Redis.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Consumer.Populate(c)
	err = args.Consumer.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Logging.Populate(c)
	err = args.Logging.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Forward.Populate(c)
	err = args.Logging.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Collector.Populate(c)
	err = args.Collector.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
	}

	args.Rabbit.Populate(c)
	err = args.Rabbit.Verify()
	if err != nil {
		return nil, cli.NewExitError(err.Error(), 1)
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

type HTTPArgs struct {
	ListenAddr string
	ListenPort int
}

func (args HTTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The HTTP listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The HTTP listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The HTTP listen address is invalid")
	return v.GetError()
}

func (args *HTTPArgs) Populate(c *cli.Context) *HTTPArgs {
	if args == nil {
		//noinspection GoAssignmentToReceiver
		args = new(HTTPArgs)
	}
	args.ListenPort = c.GlobalInt("http-port")
	args.ListenAddr = strings.TrimSpace(c.GlobalString("http-addr"))
	return args
}
