package main

import (
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"github.com/stephane-martin/mailstats/extractors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/pake"
	"github.com/urfave/cli"
)


func MakeApp() *cli.App {
	app := cli.NewApp()
	app.Name = "mailstats"
	app.Usage = "generate logs and stats from mail traffic"
	app.Flags = []cli.Flag{
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
			Name:   "out,o",
			Value:  "stdout",
			Usage:  "where to write the results [stdout, stderr, file, redis, syslog]",
			EnvVar: "MAILSTATS_OUT",
		},
		cli.StringFlag{
			Name:   "outfile",
			Value:  "/tmp/mailstats",
			Usage:  "when writing results to file, the filename",
			EnvVar: "MAILSTATS_OUTFILE",
		},
		cli.StringFlag{
			Name:   "outurl",
			Value:  "http://127.0.0.1:8081/dump",
			Usage:  "when writing results to HTTP, the url where to post results",
			EnvVar: "MAILSTATS_OUTURL",
		},
		cli.StringFlag{
			Name:   "redis-url",
			Value:  "redis://127.0.0.1:6379?db=0",
			Usage:  "redis connection URL",
			EnvVar: "MAILSTATS_REDIS_URL",
		},
		cli.StringFlag{
			Name:   "redis-results-key",
			Value:  "mailstats.results",
			Usage:  "The key for the results list in redis",
			EnvVar: "MAILSTATS_REDIS_RESULTS_KEY",
		},
		cli.StringFlag{
			Name:   "forward",
			Value:  "",
			Usage:  "specify a SMTP connection URL (eg. smtp://127.0.0.1:25) to forward all received messages",
			EnvVar: "MAILSTATS_FORWARD",
		},
		cli.StringFlag{
			Name:   "collector",
			Value:  "channel",
			Usage:  "The kind of collector to use (channel, filesystem or redis)",
			EnvVar: "MAILSTATS_COLLECTOR",
		},
		cli.IntFlag{
			Name:   "collector-size",
			Usage:  "size of the collector queue (for channel collector)",
			Value:  10000,
			EnvVar: "MAILSTATS_COLLECTOR_SIZE",
		},
		cli.StringFlag{
			Name:   "collector-dir",
			Value:  "/var/lib/mailstats",
			Usage:  "Where to store the incoming mails (for filesystem collector)",
			EnvVar: "MAILSTATS_COLLECTOR_DIRECTORY",
		},
		cli.StringFlag{
			Name:   "redis-collector-key",
			Value:  "mailstats.collector",
			Usage:  "When using redis as the collector, the key to use",
			EnvVar: "MAILSTATS_REDIS_COLLECTOR_KEY",
		},
		cli.StringFlag{
			Name:   "secret",
			Value:  "",
			Usage:  "the secret used for authentication with workers",
			EnvVar: "MAILSTATS_SECRET",
		},
		cli.IntFlag{
			Name:   "nbparsers",
			Value:  -1,
			Usage:  "how many parsers should be started",
			EnvVar: "MAILSTATS_NBPARSERS",
		},
	}
	app.Version = Version
	app.Commands = []cli.Command{
		{
			Name:   "worker",
			Usage:  "start worker",
			Action: Worker,
		},
		{
			Name:  "testauth",
			Usage: "blah",
			Action: func(c *cli.Context) error {
				check := func(e error) {
					if e != nil {
						panic(e)
					}
				}
				// pick an elliptic curve
				curve := elliptic.P521()
				// both parties should have a weak key
				//pw := []byte("zogzog")
				pw := []byte{1, 2, 3}

				// initialize sender P ("0" indicates sender)
				P, err := pake.Init(pw, 0, curve)
				check(err)

				// initialize recipient Q ("1" indicates recipient)
				Q, err := pake.Init(pw, 1, curve)
				check(err)

				// first, P sends u to Q
				err = Q.Update(P.Bytes())
				check(err) // errors will occur when any part of the process fails

				// Q computes k, sends H(k), v back to P
				err = P.Update(Q.Bytes())
				check(err)

				// P computes k, H(k), sends H(k) to Q
				err = Q.Update(P.Bytes())
				check(err)

				return nil

			},
		},
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
			Name:   "dump",
			Usage:  "start a debug HTTP server",
			Action: Dump,
		},
		{
			Name: "mbox",
			Usage: "read a mboxrd file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "filename, f",
					Usage: "the mbox file to read",
				},
			},
			Action: MBoxAction,
		},
		{
			Name:   "pdfinfo",
			Usage:  "extract metadata from PDF",
			Action: func(c *cli.Context) error {
				filename := c.String("filename")
				if filename == "" {
					return cli.NewExitError("No filename", 1)
				}
				meta, err := extractors.PDFInfo(filename)
				if err != nil {
					return cli.NewExitError(err, 2)
				}
				b, err := json.Marshal(meta)
				if err != nil {
					return cli.NewExitError(err, 2)
				}
				fmt.Println(string(b))
				return nil
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "PDF file to analyze",
				},
			},
		},
		{
			Name: "pdftotext",
			Usage: "convert PDF to text",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "filename, f",
					Usage: "PDF file to process",
				},
			},
			Action: func(c *cli.Context) error {
				f := strings.TrimSpace(c.String("filename"))
				if f != "" {
					content, err := extractors.PDFToText(f)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(content)
				}
				return nil
			},
		},
		{
			Name:  "keywords",
			Usage: "extract keywords from text",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to process",
				},
				cli.StringFlag{
					Name: "language, l",
					Usage: "language of text",
				},
			},
			Action: func(c *cli.Context) error {
				l := strings.ToLower(strings.TrimSpace(c.String("language")))
				switch l {
				case "en":
					l = "english"
				case "fr":
					l = "french"
				}
				f := strings.TrimSpace(c.String("filename"))
				if f != "" {
					content := ""
					ext := strings.ToLower(filepath.Ext(f))
					switch ext {
					case ".pdf":
						var err error
						content, err = extractors.PDFToText(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					case ".docx":
						var err error
						content, _, err = extractors.ConvertDocx(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					case ".odt":
						var err error
						content, _, err = extractors.ConvertODT(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					default:
						fil, err := os.Open(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						//noinspection GoUnhandledErrorResult
						defer fil.Close()
						c, err := ioutil.ReadAll(fil)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						content = string(c)
					}
					words := extractors.Keywords(content, nil, l)
					for _, word := range words {
						fmt.Println(word)
					}
				}
				return nil
			},
		},
		{
			Name:  "html2text",
			Usage: "convert a HTML document to text",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "url, u",
					Usage: "URL to convert",
				},
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "HTML file to convert",
				},
			},
			Action: func(c *cli.Context) error {
				f := strings.TrimSpace(c.String("filename"))
				u := strings.TrimSpace(c.String("url"))
				content := ""
				if u != "" {
					resp, err := http.Get(u)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					defer resp.Body.Close()
					c, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					content = string(c)
				} else if f != "" {
					fil, err := os.Open(f)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					//noinspection GoUnhandledErrorResult
					defer fil.Close()
					c, err := ioutil.ReadAll(fil)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					content = string(c)
				}
				content = strings.TrimSpace(content)
				if content != "" {
					t, _, _ := html2text(content)
					fmt.Println(t)
				}
				return nil
			},
		},
	}
	return app
}
