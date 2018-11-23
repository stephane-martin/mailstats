package main

import "github.com/urfave/cli"

type Args struct {
	SMTP SMTPArgs
	Milter MilterArgs
	HTTP HTTPArgs
	Redis RedisArgs
	Consumer ConsumerArgs
	Logging LoggingArgs
	QueueSize int
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

	args.QueueSize = c.GlobalInt("queue-size")
	if args.QueueSize <= 0 {
		args.QueueSize = 10000
	}

	return args, nil
}
