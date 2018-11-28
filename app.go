package main

import (
	"github.com/urfave/cli"
)

func MakeApp() *cli.App {
	app := cli.NewApp()
	app.Name = "mailstats"
	app.Usage = "generate logs and stats from mail traffic"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "queue-size,q",
			Usage:  "size of the internal message queue",
			Value:  10000,
			EnvVar: "MAILSTATS_QUEUE_SIZE",
		},
		cli.StringFlag{
			Name:   "http-addr",
			Usage:  "HTTP listen address",
			Value:  "127.0.0.1",
			EnvVar: "MAILSTATS_HTTP_ADDR",
		},
		cli.IntFlag{
			Name:   "http-port",
			Usage:  "HTTP liten port",
			Value:  8080,
			EnvVar: "MAILSTATS_HTTP_PORT",
		},
		cli.BoolFlag{
			Name:   "inetd",
			Usage:  "Ignore the milter and SMTP port options and use the socket passed by inetd instead",
			EnvVar: "MAILSTATS_INETD",
		},
		cli.BoolFlag{
			Name:   "syslog",
			Usage:  "write logs to syslog instead of stderr",
			EnvVar: "MAILSTATS_SYSLOG",
		},
		cli.StringFlag{
			Name:   "loglevel",
			Value:  "info",
			Usage:  "logging level",
			EnvVar: "MAILSTATS_LOGLEVEL",
		},
		cli.StringFlag{
			Name: "out,o",
			Value: "stdout",
			Usage: "where to write the results [stdout, stderr, file, redis, syslog]",
			EnvVar: "MAILSTATS_OUT",
		},
		cli.StringFlag{
			Name: "outfile",
			Value: "/tmp/mailstats",
			Usage: "when writing results to file, the filename",
			EnvVar: "MAILSTATS_OUTFILE",
		},
		cli.StringFlag{
			Name: "redis-url",
			Value: "redis://127.0.0.1:6379?db=0",
			Usage: "redis connection URL",
			EnvVar: "MAILSTATS_REDIS_URL",
		},
		cli.StringFlag{
			Name: "redis-results-key",
			Value: "mailstats",
			Usage: "The key for the results list in redis",
			EnvVar: "MAILSTATS_REDIS_RESULTS_KEY",
		},
		cli.StringFlag{
			Name: "forward",
			Value: "",
			Usage: "specify a SMTP connection URL (eg. smtp://127.0.0.1:25) to forward all received messages",
			EnvVar: "MAILSTATS_FORWARD",
		},

	}
	app.Version = Version
	app.Commands = []cli.Command{
		{
			Name:   "milter",
			Usage:  "start as a Postfix milter",
			Action: Milter,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "laddr,l",
					Usage:  "milter address to listen on",
					Value:  "127.0.0.1",
					EnvVar: "MAILSTATS_MILTER_LISTENADDR",
				},
				cli.IntFlag{
					Name:   "lport,p",
					Usage:  "milter port to listen on",
					Value:  3333,
					EnvVar: "MAILSTATS_MILTER_LISTENPORT",
				},
			},
		},
		{
			Name:   "smtp",
			Usage:  "start as a SMTP service",
			Action: SMTP,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "laddr,l",
					Usage:  "smtp address to listen on",
					Value:  "127.0.0.1",
					EnvVar: "MAILSTATS_SMTP_LISTENADDR",
				},
				cli.IntFlag{
					Name:   "lport,p",
					Usage:  "smtp port to listen on",
					Value:  3333,
					EnvVar: "MAILSTATS_SMTP_LISTENPORT",
				},
				cli.IntFlag{
					Name:   "max-size",
					Usage:  "Maximum incoming message size in bytes",
					Value:  60 * 1024 * 1024,
					EnvVar: "MAILTSATS_SMTP_MAXSIZE",
				},
				cli.IntFlag{
					Name:  "max-idle",
					Usage: "Maximum idle time in seconds",
					Value: 300,
				},
			},
		},
		{
			Name: "dump",
			Usage: "start a debug HTTP server",
			Action: Dump,
		},

	}
	return app
}
