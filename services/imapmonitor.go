package services

import (
	"context"
	"fmt"
	"github.com/ahmetb/go-linq"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	imap.CharsetReader = charset.Reader
}

func IMAPMonitorAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		return err
	}
	logger := args.Logging.Build()
	uri := strings.TrimSpace(c.String("uri"))
	if uri == "" {
		return nil
	}
	u, err := url.Parse(uri)
	if err != nil {
		return cli.NewExitError("Invalid URI", 1)
	}
	if u.Scheme != "imap" && u.Scheme != "imaps" {
		return cli.NewExitError("Scheme must be imap or imaps", 1)
	}

	host, portS, err := net.SplitHostPort(u.Host)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Error splitting host/port: %s", err.Error()), 1)
	}
	port, err := strconv.ParseInt(portS, 10, 32)
	if err != nil {
		return cli.NewExitError("Port is not a number", 1)
	}

	username := strings.TrimSpace(u.User.Username())
	password, _ := u.User.Password()
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return cli.NewExitError("Specify username and password", 1)
	}

	// Connect to server
	var clt *client.Client
	if u.Scheme == "imap" {
		clt, err = client.Dial(u.Host)
	} else {
		clt, err = client.DialTLS(u.Host, nil)
	}
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to dial: %s", err.Error()), 1)
	}

	collector, err := collectors.NewCollector(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}

	forwarder, err := forwarders.Build(args.Forward, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build forwarder: %s", err), 3)
	}

	consumer, err := consumers.MakeConsumer(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update, 256)
	clt.Updates = updates

	defer func() {
		logger.Info("Logging out of IMAP")
		_ = clt.Logout()
	}()

	// Login
	err = clt.Login(username, password)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to authenticate: %s", err.Error()), 1)
	}

	// Enable compression if possible
	comp := compress.NewClient(clt)
	if ok, err := comp.SupportCompress(compress.Deflate); err != nil {
		return cli.NewExitError(fmt.Sprintf("IMAP support compress error: %s", err.Error()), 1)
	} else if ok {
		if err := comp.Compress(compress.Deflate); err != nil {
			return cli.NewExitError(fmt.Sprintf("IMAP compress error: %s", err.Error()), 1)
		} else {
			logger.Info("IMAP compression", "enabled", comp.IsCompress())
		}
	}

	// Select box
	box, err := clt.Select(strings.Trim(u.Path, "/"), false)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to select box: %s", err.Error()), 1)
	}
	logger.Info(
		"Mailbox selected",
		"name", box.Name,
		"nb_messages", box.Messages,
	)

	max, err := getMaxUID(clt)
	logger.Info("Max UID of messages stored in box", "max", max)

	idleClient := idle.NewClient(clt)
	idleClient.LogoutTimeout = 24 * time.Minute

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())
	stopIdle := make(chan struct{}, 256)

	imapMsgs := make(chan *imap.Message)
	section := &imap.BodySectionName{}

	go func() {
		for sig := range sigchan {
			logger.Info("Signal received", "signal", sig.String())
			cancel()
		}
	}()

	theparser := parser.NewParser(logger)

	var collG errgroup.Group
	collG.Go(func() error {
		return collector.Start()
	})

	g, ctx := errgroup.WithContext(gctx)

	g.Go(func() error {
		err := StartHTTP(ctx, args.HTTP, collector, forwarder, logger)
		logger.Debug("StartHTTP has returned", "error", err)
		return err
	})

	if args.Secret != nil {
		g.Go(func() error {
			err := StartMaster(ctx, args.HTTP, args.Secret, collector, consumer, logger)
			logger.Debug("StartMaster has returned", "error", err)
			return err
		})
	}

	g.Go(func() error {
		err := parser.ParseMails(ctx, collector, theparser, consumer, args.NbParsers, logger)
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

	g.Go(func() error {
		defer close(imapMsgs)

		for {
			var uids []uint32
			var err error
		WaitNewMessages:
			for {
				uids, err = getNewUIDs(clt, max)
				if err != nil {
					return err
				}
				if len(uids) > 0 && uids[0] != max {
					break WaitNewMessages
				}

				logger.Info("No new messages, going to IDLE")

				// reset stopIdle
			ConsumeStopIdle:
				for {
					select {
					case _, ok := <-stopIdle:
						if !ok {
							return nil
						}
					case <-ctx.Done():
						return nil
					default:
						break ConsumeStopIdle
					}
				}

				err = idleClient.IdleWithFallback(stopIdle, 0)
				if err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return nil
				default:
				}
			}

			max = linq.From(uids).Max().(uint32)
			logger.Info("Max UID of messages stored in box", "max", max)
			logger.Info("New messages", "uids", uids)
			set := new(imap.SeqSet)
			set.AddNum(uids...)
			// fetch new messages
			newImapMsgs := make(chan *imap.Message)
			g.Go(func() error {
				for msg := range newImapMsgs {
					imapMsgs <- msg
				}
				return nil
			})
			err = clt.UidFetch(set, []imap.FetchItem{section.FetchItem()}, newImapMsgs)
			if err != nil {
				return err
			}
		}
	})

	g.Go(func() error {
		defer func() {
			logger.Info("No more messages")
		}()
		for msg := range imapMsgs {
			body, err := ioutil.ReadAll(msg.GetBody(section))
			if err != nil {
				logger.Warn("Error reading message from IMAP", "error", err)
				continue
			}
			incoming := &models.IncomingMail{
				BaseInfos: models.BaseInfos{
					UID:          utils.NewULID(),
					TimeReported: time.Now(),
					Family:       u.Scheme,
					Port:         int(port),
					Host:         host,
				},
				Data: body,
			}
			err = collectors.CollectAndForward(ctx.Done(), incoming, collector, forwarder)
			if err != nil {
				return err
			}
		}
		return nil
	})

	g.Go(func() error {
		defer func() {
			close(stopIdle)
		}()
		// Listen for updates
		for {
			select {
			case <-ctx.Done():
				return nil
			case update, ok := <-updates:
				if !ok {
					return nil
				}
				switch up := update.(type) {
				case *client.StatusUpdate:
					logger.Info("Status update",
						"info", up.Status.Info,
						"type", up.Status.Type,
						"code", up.Status.Code,
						"tag", up.Status.Tag,
						"error", up.Status.Err(),
					)
				case *client.MailboxUpdate:
					logger.Info("Mailbox update",
						"name", up.Mailbox.Name,
						"nb_messages", up.Mailbox.Messages,
					)
					stopIdle <- struct{}{}

				case *client.ExpungeUpdate:
					logger.Info("Expunge update", "seqnum", up.SeqNum)
				case *client.MessageUpdate:
					logger.Info("Message update", "seqnum", up.Message.SeqNum)
				}
			}
		}
	})

	err = g.Wait()
	_ = collector.Close()
	_ = theparser.Close()
	_ = forwarder.Close()
	_ = consumer.Close()
	_ = collG.Wait()

	if err != nil {
		logger.Info("IMAP Monitor Wait", "error", err)
	}
	return nil

}

func getMaxUID(clt *client.Client) (uint32, error) {
	uids, err := getNewUIDs(clt, 0)
	if err != nil {
		return 0, err
	}
	return linq.From(uids).Max().(uint32), nil
}

func getNewUIDs(clt *client.Client, previousUID uint32) ([]uint32, error) {
	criteria := imap.NewSearchCriteria()
	criteria.Uid = new(imap.SeqSet)
	criteria.Uid.AddRange(previousUID+1, 0)
	uids, err := clt.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed list messages UIDs: %s", err.Error())
	}
	return uids, nil
}
