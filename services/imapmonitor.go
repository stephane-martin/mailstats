package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/ahmetb/go-linq"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-compress"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"go.uber.org/fx"
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

type IMAPMonitor struct {
	URI          string
	clt          *client.Client
	u            *url.URL
	login        string
	password     string
	logger       log15.Logger
	updates      chan client.Update
	imapMessages chan *imap.Message
	collector    collectors.Collector
	forwarder    forwarders.Forwarder
	host         string
	port         int
}

var IMAPMonitorService = fx.Provide(func(lc fx.Lifecycle, c *cli.Context, collector collectors.Collector, forwarder forwarders.Forwarder, logger log15.Logger) (*IMAPMonitor, error) {
	s, err := NewIMAPMonitor(c, collector, forwarder, logger)
	if err != nil {
		return nil, err
	}
	if lc != nil {
		utils.Append(lc, s, logger)
	}
	return s, nil
})

func NewIMAPMonitor(c *cli.Context, collector collectors.Collector, forwarder forwarders.Forwarder, logger log15.Logger) (*IMAPMonitor, error) {
	uri := strings.TrimSpace(c.String("uri"))
	if uri == "" {
		return nil, errors.New("empty IMAP URI")
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, errors.New("invalid IMAP URI")
	}
	if u.Scheme != "imap" && u.Scheme != "imaps" {
		return nil, errors.New("scheme must be imap or imaps")
	}

	host, portS, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("error splitting host/port: %s", err)
	}
	port, err := strconv.ParseInt(portS, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("port is not a number: %s", err)
	}

	username := strings.TrimSpace(u.User.Username())
	password, _ := u.User.Password()
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, errors.New("specify username and password")
	}

	logger.Info("Monitoring IMAP mailbox", "host", host, "port", port)

	return &IMAPMonitor{
		URI:          uri,
		u:            u,
		login:        username,
		password:     password,
		logger:       logger,
		updates:      make(chan client.Update, 256),
		imapMessages: make(chan *imap.Message),
		collector:    collector,
		forwarder:    forwarder,
		host:         host,
		port:         int(port),
	}, nil
}

func (m *IMAPMonitor) Name() string {
	return "IMAPMonitor"
}

func (m *IMAPMonitor) Prestart() error {
	// Connect to server
	var clt *client.Client
	var err error

	if m.u.Scheme == "imap" {
		clt, err = client.Dial(m.u.Host)
	} else {
		clt, err = client.DialTLS(m.u.Host, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}
	m.clt = clt

	// Create a channel to receive mailbox updates
	m.clt.Updates = m.updates

	// Login
	err = clt.Login(m.login, m.password)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %s", err)
	}

	// Enable compression if possible
	comp := compress.NewClient(clt)
	if ok, err := comp.SupportCompress(compress.Deflate); err != nil {
		return fmt.Errorf("IMAP support compress error: %s", err)
	} else if ok {
		if err := comp.Compress(compress.Deflate); err != nil {
			return fmt.Errorf("IMAP compress error: %s", err)
		} else {
			m.logger.Info("IMAP compression", "enabled", comp.IsCompress())
		}
	}

	// Select box
	box, err := clt.Select(strings.Trim(m.u.Path, "/"), false)
	if err != nil {
		return fmt.Errorf("failed to select box: %s", err)
	}
	m.logger.Info(
		"Mailbox selected",
		"name", box.Name,
		"nb_messages", box.Messages,
	)

	return nil
}

func (m *IMAPMonitor) Close() error {
	if m.clt != nil {
		return m.clt.Logout()
	}
	return nil
}

func (m *IMAPMonitor) Start(ctx context.Context) error {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	g, lctx := errgroup.WithContext(ctx)
	stopIdle := make(chan struct{}, 256)

	g.Go(func() error {
		return m.fetch(lctx, stopIdle)
	})

	g.Go(func() error {
		return m.collect(lctx)
	})

	g.Go(func() error {
		m.reactUpdates(lctx, stopIdle)
		return nil
	})

	return g.Wait()
}

func (m *IMAPMonitor) fetch(ctx context.Context, stopIdle chan struct{}) error {
	defer close(m.imapMessages)

	idleClient := idle.NewClient(m.clt)
	idleClient.LogoutTimeout = 24 * time.Minute

	max, err := getMaxUID(m.clt)
	if err != nil {
		return err
	}

	m.logger.Info("Max UID of messages stored in box", "max", max)

	section := &imap.BodySectionName{}

	for {
		var uids []uint32
		var err error

	WaitNewMessages:
		for {
			uids, err = getNewUIDs(m.clt, max)
			if err != nil {
				return err
			}
			if len(uids) > 0 && uids[0] != max {
				break WaitNewMessages
			}

			m.logger.Info("No new messages, going to IDLE")

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
		m.logger.Info("Max UID of messages stored in box", "max", max)
		m.logger.Info("New messages", "uids", uids)
		set := new(imap.SeqSet)
		set.AddNum(uids...)
		// fetch new messages
		newImapMsgs := make(chan *imap.Message)
		go func() {
			for msg := range newImapMsgs {
				m.imapMessages <- msg
			}
		}()
		err = m.clt.UidFetch(set, []imap.FetchItem{section.FetchItem()}, newImapMsgs)
		if err != nil {
			return err
		}
	}
}

func (m *IMAPMonitor) collect(ctx context.Context) error {
	section := &imap.BodySectionName{}

	for msg := range m.imapMessages {
		body, err := ioutil.ReadAll(msg.GetBody(section))
		if err != nil {
			m.logger.Warn("Error reading message from IMAP", "error", err)
			continue
		}
		incoming := &models.IncomingMail{
			BaseInfos: models.BaseInfos{
				UID:          utils.NewULID(),
				TimeReported: time.Now(),
				Family:       m.u.Scheme,
				Port:         int(m.port),
				Host:         m.host,
			},
			Data: body,
		}
		err = collectors.CollectAndForward(ctx.Done(), incoming, m.collector, m.forwarder)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *IMAPMonitor) reactUpdates(ctx context.Context, stopIdle chan struct{}) {
	defer close(stopIdle)
	// Listen for updates
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-m.updates:
			if !ok {
				return
			}
			switch up := update.(type) {
			case *client.StatusUpdate:
				m.logger.Info("Status update",
					"info", up.Status.Info,
					"type", up.Status.Type,
					"code", up.Status.Code,
					"tag", up.Status.Tag,
					"error", up.Status.Err(),
				)
			case *client.MailboxUpdate:
				m.logger.Info("Mailbox update",
					"name", up.Mailbox.Name,
					"nb_messages", up.Mailbox.Messages,
				)
				stopIdle <- struct{}{}

			case *client.ExpungeUpdate:
				m.logger.Info("Expunge update", "seqnum", up.SeqNum)
			case *client.MessageUpdate:
				m.logger.Info("Message update", "seqnum", up.Message.SeqNum)
			}
		}
	}
}

func IMAPMonitorAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		err = fmt.Errorf("error validating cli arguments: %s", err)
		return cli.NewExitError(err.Error(), 1)
	}

	logger := logging.NewLogger(args)
	withRedis := args.RedisRequired()
	invoke := fx.Invoke(func(h *HTTPServer, m *HTTPMasterServer, s *IMAPMonitor) {
		// bootstrap the application
	})
	app := Builder(c, args, invoke, withRedis, logger)
	app.Run()
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
	linq.From(uids).SortT(func(i, j uint32) bool {return i < j}).ToSlice(&uids)
	return uids, nil
}
