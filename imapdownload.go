package main

import (
	"context"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap/client"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"math"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func IMAPDownloadAction(c *cli.Context) error {
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

	maxDownloads := c.Uint64("max")
	if maxDownloads == 0 {
		maxDownloads = math.MaxUint32
	}
	if maxDownloads > math.MaxUint32 {
		maxDownloads = math.MaxUint32
	}

	if c.GlobalBool("geoip") {
		err := utils.InitGeoIP(c.GlobalString("geoip-database-path"))
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error loading GeoIP database: %s", err), 1)
		}
		//noinspection GoUnhandledErrorResult
		defer utils.CloseGeoIP()
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

	//noinspection GoUnhandledErrorResult
	defer clt.Logout()

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

	// List mailboxes
	//mailboxes := make(chan *imap.MailboxInfo, 10)
	//done := make(chan error, 1)
	//go func() {
	//	done <- c.List("", "*", mailboxes)
	//}()

	// Select box
	mbox, err := clt.Select(strings.Trim(u.Path, "/"), false)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to select box: %s", err.Error()), 1)
	}
	if mbox.Messages == 0 {
		return nil
	}
	var nbDownloads uint32 = uint32(maxDownloads)
	if nbDownloads > mbox.Messages {
		nbDownloads = mbox.Messages
	}

	seqset := new(imap.SeqSet)

	seqset.AddRange(1, mbox.Messages)

	imapMsgs := make(chan *imap.Message)
	section := &imap.BodySectionName{}


	collector, err := collectors.NewChanCollector(args.Collector.CollectorSize, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}

	forwarder := forwarders.DummyForwarder{}

	consumer, err := consumers.MakeConsumer(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())

	go func() {
		for sig := range sigchan {
			logger.Info("Signal received", "signal", sig.String())
			cancel()
			_ = clt.Terminate()
		}
	}()

	g, ctx := errgroup.WithContext(gctx)

	parser := NewParser(logger)
	//noinspection GoUnhandledErrorResult
	defer parser.Close()

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, args.NbParsers, logger)
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

	g.Go(func() error {
		criteria := imap.NewSearchCriteria()
		criteria.SeqNum = new(imap.SeqSet)
		criteria.SeqNum.AddRange(mbox.Messages + 1 - nbDownloads, mbox.Messages)
		uids, err := clt.UidSearch(criteria)
		if err != nil {
			return err
		}
		set := new(imap.SeqSet)
		set.AddNum(uids...)
		return clt.UidFetch(set, []imap.FetchItem{section.FetchItem()}, imapMsgs)
	})

	g.Go(func() error {
		defer func() {
			logger.Info("No more messages")
			_ = collector.Close()
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
			err = collector.PushCtx(ctx, incoming)
			if err != nil {
				return err
			}
		}
		return nil
	})

	err = g.Wait()
	if err != nil && err != context.Canceled {
		logger.Warn("goroutine group error", "error", err)
	}

	return nil
}
