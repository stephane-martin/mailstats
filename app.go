package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/russross/blackfriday"
	"github.com/stephane-martin/mailstats/actions"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/phishtank"
	"github.com/stephane-martin/mailstats/services"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)


func MakeApp() *cli.App {
	app := cli.NewApp()
	app.Name = "mailstats"
	app.Usage = "generate logs and stats from mail traffic"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "api-listen-addr",
			Usage:  "HTTP API listen address",
			Value:  "127.0.0.1:8080",
			EnvVar: "MAILSTATS_API_ADDR",
		},
		cli.StringFlag{
			Name: "master-listen-addr",
			Usage: "When using external workers, the listen address for the master",
			Value: "127.0.0.1:8081",
			EnvVar: "MAILSTATS_MASTER_ADDR",
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
			Usage:  "where to write the results [stdout, stderr, file, http, redis, syslog, rabbitmq, kafka]",
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
			Usage:  "HTTP or SMTP connection URL (eg. smtp://127.0.0.1:25) to forward all received messages",
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
			Name: "cache-dir",
			Value: "/var/lib/mailstats",
			Usage: "Cache directory for HTTP requests",
			EnvVar: "MAILSTATS_CACHE_DIR",
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
			Usage:  "the secret used for authentication between master and workers",
			EnvVar: "MAILSTATS_SECRET",
		},
		cli.IntFlag{
			Name:   "nbparsers",
			Value:  -1,
			Usage:  "how many parsers should be started (-1 for number of CPUs)",
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
		cli.StringFlag{
			Name: "geoip-database-path",
			Value: "/var/lib/mailstats/GeoLite2-City/GeoLite2-City.mmdb",
			Usage: "path to the GeoIP lite database",
			EnvVar: "MAILSTATS_GEOIP_DATABASE_PATH",
		},
		cli.BoolFlag{
			Name: "geoip",
			Usage: "enable geolocation of IP addresses",
			EnvVar: "MAILSTATS_GEOIP",
		},
		cli.StringSliceFlag{
			Name: "broker",
			Usage: "kafka broker, for kafka output (can be specified multiple times)",
			EnvVar: "MAILSTATS_KAFKA_BROKER",
		},
		cli.StringFlag{
			Name: "topic",
			Usage: "kafka topic, for kafka output",
			Value: "mailstats.results",
			EnvVar: "MAILSTATS_KAFKA_TOPIC",
		},
		cli.StringSliceFlag{
			Name: "elasticsearch-url",
			Usage: "URL for Elasticsearch node",
			EnvVar: "MAILSTATS_ELASTICSEARCH_URL",
		},
		cli.StringFlag{
			Name: "elasticsearch-index-name",
			Usage: "Elasticsearch index where to store results",
			Value: "mailstats",
			EnvVar: "MAILSTATS_ELASTICSEARCH_INDEX_NAME",
		},
		cli.BoolFlag{
			Name: "no-dkim",
			Usage: "Do not perform DKIM validation",
			EnvVar: "MAILSTATS_NO_DKIM",
		},
		cli.BoolFlag{
			Name: "phishtank",
			Usage: "Identify phishing URLs with Phishtank",
			EnvVar: "MAILSTATS_PHISHTANK",
		},
		cli.StringFlag{
			Name: "phishtank-appkey",
			Usage: "The phishtank application key",
			EnvVar: "MAILSTATS_PHISHTANK_APPKEY",
			Value: "",
		},

	}
	app.Version = Version
	app.Commands = []cli.Command{
		{
			Name: "args",
			Usage: "print args",
			Action: func(c *cli.Context) error {
				args, err := arguments.GetArgs(c)
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
				b, err := json.MarshalIndent(args, "", "  ")
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
				fmt.Println(string(b))
				return nil
			},
		},
		{
			Name:   "worker",
			Usage:  "start external worker",
			Action: services.WorkerAction,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "master",
					Usage: "Master address",
					Value: "127.0.0.1:8081",
					EnvVar: "MAILSTATS_MASTER_ADDR",
				},
			},
		},
		{
			Name:   "milter",
			Usage:  "start as a Postfix milter",
			Action: services.MilterAction,
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
			Action: services.SMTPAction,
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
					EnvVar: "MAILSTATS_SMTP_MAXSIZE",
				},
				cli.IntFlag{
					Name:  "max-idle",
					Usage: "Maximum idle time in seconds",
					Value: 300,
					EnvVar: "MAILSTATS_SMTP_MAXIDLE",
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
					EnvVar: "MAILSTATS_MBOX_FILE",
				},
			},
			Action: actions.MBoxAction,
		},
		{
			Name:  "maildir",
			Usage: "read a maildir directory",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "directory, d",
					Usage: "the maildir to read",
					EnvVar: "MAILSTATS_MAILDIR",
				},
			},
			Action: actions.MaildirAction,
		},

		{
			Name:  "imapdownload",
			Usage: "Read and parse IMAP box",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "uri",
					Usage: "IMAP connection uri",
					Value: "imaps://user:pass@example.org:993/INBOX",
					EnvVar: "MAILSTATS_IMAP_DOWNLOAD_URI",
				},
				cli.Uint64Flag{
					Name: "max",
					Usage: "Download max massages at most",
					Value: 0,
					EnvVar: "MAILSTATS_IMAP_DOWNLOAD_MAX",
				},
			},
			Action: actions.IMAPDownloadAction,
		},
		{
			Name:  "imapmonitor",
			Usage: "Monitor an IMAP box for new messages",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "uri",
					Usage: "IMAP connection uri",
					Value: "imaps://user:pass@example.org:993/INBOX",
					EnvVar: "MAILSTATS_IMAP_MONITOR_URI",
				},
			},
			Action: services.IMAPMonitorAction,
		},
		{
			Name:  "metadata",
			Usage: "extract metadata from document",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to analyze",
				},
			},
			Action: func(c *cli.Context) error {
				filename := c.String("filename")
				if filename == "" {
					return nil
				}
				extension := strings.ToLower(filepath.Ext(filename))
				switch extension {
				case ".pdf":
					meta, err := extractors.PDFInfo(filename, nil)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(utils.JSONString(meta))
				case ".docx", ".docm":
					_, meta, _, err := extractors.ConvertDocx(filename)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(utils.JSONString(meta))
				case ".odt":
					_, meta, err := extractors.ConvertODT(filename)
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(utils.JSONString(meta))
				case ".doc":
					tool := extractors.NewExifTool(nil)
					if tool == nil {
						return cli.NewExitError("exiftool not found", 2)
					}
					err := tool.Prestart()
					if err != nil {
						return cli.NewExitError(err.Error(), 2)
					}
					//noinspection GoUnhandledErrorResult
					defer tool.Close()
					meta, err := tool.ExtractFromFile(filename, nil,"-FlashPix:All")
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(utils.JSONString(meta))
				case ".jpg", ".jpeg", ".png", ".tiff", ".gif", ".webp":
					tool := extractors.NewExifTool(nil)
					if tool == nil {
						return cli.NewExitError("exiftool not found", 2)
					}
					err := tool.Prestart()
					if err != nil {
						return cli.NewExitError(err.Error(), 2)
					}
					//noinspection GoUnhandledErrorResult
					defer tool.Close()
					meta, err := tool.ExtractFromFile(filename, nil, "-EXIF:All")
					if err != nil {
						return cli.NewExitError(err, 2)
					}
					fmt.Println(utils.JSONString(meta))
				}
				return nil
			},
		},
		{
			Name:  "totext",
			Usage: "Convert document to text",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "filename, f",
					Usage: "File to process",
				},
			},
			Action: func(c *cli.Context) error {
				filename := strings.TrimSpace(c.String("filename"))
				if filename == "" {
					return nil
				}

				extension := strings.ToLower(filepath.Ext(filename))
				switch extension {
				case ".pdf":
					content, err := extractors.PDFToText(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(content)
				case ".docx", ".docm":
					content, _, _, err := extractors.ConvertDocx(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(content)
				case ".odt":
					content, _, err := extractors.ConvertODT(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(content)
				case ".doc":
					content, err := extractors.ConvertDoc(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(content)
				case ".html":
					f, err := os.Open(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					//noinspection GoUnhandledErrorResult
					defer f.Close()
					c, err := ioutil.ReadAll(f)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					content, _, _ := extractors.HTML2Text(string(c))
					fmt.Println(content)
				case ".md":
					f, err := os.Open(filename)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					//noinspection GoUnhandledErrorResult
					defer f.Close()
					_, _ = io.Copy(os.Stdout, f)
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
					t, _, err := utils.GuessReader(fname, f)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					fmt.Println(t.MIME.Value)
				}
				return nil
			},
		},
		{
			Name: "phishtank",
			Usage: "Download and parse phishtank feed",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "appkey",
					Usage: "phishtank application key",
					Value: "",
				},
			},
			Action: func(c *cli.Context) error {
				appkey := c.String("appkey")
				cacheDir := c.GlobalString("cache-dir")
				logger := log15.New()
				logger.SetHandler(log15.StderrHandler)
				entries, errs := phishtank.Download(context.Background(), cacheDir, appkey, logger)
				tree, err := phishtank.BuildTree(context.Background(), entries, errs, logger)
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
				tree.Walk(func(url string, entries []*models.PhishtankEntry) bool {
					if len(entries) > 1 {
						fmt.Println(len(entries), url)
					}
					return true
				})
				return nil
			},
		},
		{
			Name: "download-geoip",
			Usage: "Download GeoIP lite database",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "directory, d",
					Usage: "the destination directory",
					Value: "/var/lib/mailstats",
				},
				cli.StringFlag{
					Name: "url, u",
					Usage: "the database URL",
					Value: utils.GeoIPURL,
				},
			},
			Action: func(c *cli.Context) error {
				err := utils.DownloadGeoIP(c.String("directory"), c.String("url"))
				if err != nil {
					return cli.NewExitError(fmt.Sprintf("Error downloading database: %s", err), 1)
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
						f, err := os.Open(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						//noinspection GoUnhandledErrorResult
						defer f.Close()
						c, err := ioutil.ReadAll(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						content, _, _ = extractors.HTML2Text(string(c))

					case ".md":
						f, err := os.Open(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						//noinspection GoUnhandledErrorResult
						defer f.Close()
						c, err := ioutil.ReadAll(f)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						html := string(blackfriday.Run(c))
						content, _, _ = extractors.HTML2Text(html)

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
					words, phrases := extractors.Keywords(content, nil, l)
					for _, word := range words {
						fmt.Println(word)
					}
					for _, phrase := range phrases {
						fmt.Println(phrase)
					}

				}
				return nil
			},
		},
	}
	return app
}
