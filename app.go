package main

import (
	"encoding/json"
	"fmt"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func ijson(obj interface{}) string {
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

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
			Usage:  "where to write the results [stdout, stderr, file, redis, syslog, rabbitmq]",
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
			Usage:  "The kind of collector to use (channel, filesystem, redis or rabbitmq)",
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
		cli.StringFlag{
			Name:   "rabbitmq-uri",
			Value:  "amqp://mailstats:mailstatspass@127.0.0.1:5672/vhmailstats",
			Usage:  "the RabbitMQ URI",
			EnvVar: "MAILSTATS_RABBITMQ_URI",
		},
		cli.StringFlag{
			Name:   "rabbitmq-collector-exchange",
			Value:  "collector.exchange",
			Usage:  "the RabbitMQ exchange to use for the RabbitMQ collector",
			EnvVar: "MAILSTATS_RABBITMQ_COLLECTOR_EXCHANGE",
		},
		cli.StringFlag{
			Name:   "rabbitmq-collector-queue",
			Value:  "collector.queue",
			Usage:  "the RabbitMQ queue to use for the RabbitMQ collector",
			EnvVar: "MAILSTATS_RABBITMQ_COLLECTOR_QUEUE",
		},
		cli.StringFlag{
			Name:   "rabbitmq-results-exchange",
			Value:  "results.exchange",
			Usage:  "the RabbitMQ exchange to use to push results to RabbitMQ",
			EnvVar: "MAILSTATS_RABBITMQ_RESULTS_EXCHANGE",
		},
		cli.StringFlag{
			Name:   "rabbitmq-results-exchange-type",
			Value:  "direct",
			Usage:  "the RabbitMQ exchange type to use to push results to RabbitMQ",
			EnvVar: "MAILSTATS_RABBITMQ_RESULTS_EXCHANGE_TYPE",
		},
		cli.StringFlag{
			Name:   "rabbitmq-results-routing-key",
			Value:  "results",
			Usage:  "the RabbitMQ routing key to use to push results to RabbitMQ",
			EnvVar: "MAILSTATS_RABBITMQ_RESULTS_ROUTING_KEY",
		},
	}
	app.Version = Version
	app.Commands = []cli.Command{
		{
			Name:   "worker",
			Usage:  "start worker",
			Action: WorkerAction,
		},
		{
			Name:   "milter",
			Usage:  "start as a Postfix milter",
			Action: MilterAction,
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
			Action: SMTPAction,
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
			Action: DumpAction,
		},
		{
			Name:  "mbox",
			Usage: "read a mboxrd file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "the mbox file to read",
				},
			},
			Action: MBoxAction,
		},
		{
			Name:  "imapdownload",
			Usage: "Read and parse IMAP box",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "uri",
					Usage: "IMAP connection uri",
					Value: "imaps://user:pass@example.org:993/INBOX",
				},
			},
			Action: IMAPDownloadAction,
		},
		{
			Name:  "metadata",
			Usage: "extract metadata from document",
			Action: func(c *cli.Context) error {
				filename := c.String("filename")
				if filename == "" {
					return cli.NewExitError("No filename", 1)
				}
				extension := strings.ToLower(filepath.Ext(filename))
				switch extension {
				case ".pdf":
					meta, err := extractors.PDFInfo(filename)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(ijson(meta))
				case ".docx", ".docm":
					_, meta, _, err := extractors.ConvertDocx(filename)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(ijson(meta))
				case ".odt":
					_, meta, err := extractors.ConvertODT(filename)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(ijson(meta))
				case ".doc":
					tool, err := extractors.NewExifToolWrapper()
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					//noinspection GoUnhandledErrorResult
					defer tool.Close()
					meta, err := tool.ExtractFromFile(filename, "-FlashPix:All")
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(ijson(meta))
				case ".jpg", ".jpeg", ".png", ".tiff", ".gif", ".webp":
					tool, err := extractors.NewExifToolWrapper()
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					//noinspection GoUnhandledErrorResult
					defer tool.Close()
					meta, err := tool.ExtractFromFile(filename, "-EXIF:All")
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(ijson(meta))
				}
				return nil
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to analyze",
				},
			},
		},
		{
			Name:  "pdftotext",
			Usage: "convert PDF to text",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to process",
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
			Name:  "mimetype",
			Usage: "Detect MIME type of file",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to process",
				},
			},
			Action: func(c *cli.Context) error {
				fname := strings.TrimSpace(c.String("filename"))
				if fname != "" {
					f, err := os.Open(fname)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					//noinspection GoUnhandledErrorResult
					defer f.Close()
					t, _, err := utils.GuessReader(f)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(t.MIME.Value)
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
					Name:  "language, l",
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
					case ".docx", ".docm":
						var err error
						content, _, _, err = extractors.ConvertDocx(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					case ".doc":
						var err error
						content, err = extractors.ConvertDoc(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					case ".odt":
						var err error
						content, _, err = extractors.ConvertODT(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
					case ".html":
						// TODO
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
