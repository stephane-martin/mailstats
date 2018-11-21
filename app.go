package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func MakeApp() *cli.App {
	app := cli.NewApp()
	app.Name = "mailstats"
	app.Usage = "generate logs and stats from SMTP traffic"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "queuesize,q",
			Usage:  "size of the internal message queue",
			Value:  10000,
			EnvVar: "MAILSTATS_QUEUE_SIZE",
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
					Usage:  "address to listen on",
					Value:  "127.0.0.1",
					EnvVar: "MAILSTATS_MILTER_LISTENADDR",
				},
				cli.IntFlag{
					Name:   "lport,p",
					Usage:  "port to listen on",
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
					Usage:  "address to listen on",
					Value:  "127.0.0.1",
					EnvVar: "MAILSTATS_SMTP_LISTENADDR",
				},
				cli.IntFlag{
					Name:   "lport,p",
					Usage:  "port to listen on",
					Value:  3333,
					EnvVar: "MAILSTATS_SMTP_LISTENPORT",
				},
				cli.IntFlag{
					Name:   "maxsize",
					Usage:  "Maximum incoming message size in bytes",
					Value:  60 * 1024 * 1024,
					EnvVar: "MAILTSATS_SMTP_MAXSIZE",
				},
				cli.IntFlag{
					Name:  "maxidle",
					Usage: "Maximum idle time in seconds",
					Value: 300,
				},
			},
		},
		{
			Name:  "words",
			Usage: "extract words",
			Action: func(c *cli.Context) error {
				bag := make(map[string]int)
				bagOfWords(c.Args().Get(0), bag)
				for word, count := range bag {
					fmt.Fprintf(os.Stdout, "%s => %d\n", word, count)
				}
				return nil
			},
		},
	}
	return app
}
