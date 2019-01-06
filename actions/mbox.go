package actions

import (
	"bytes"
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
	"io"
	"os"
	"strings"
	"time"

	"github.com/blabber/mbox"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/models"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

func MBoxAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		err = fmt.Errorf("error validating mbox cli arguments: %s", err)
		return cli.NewExitError(err.Error(), 1)
	}
	logger := logging.NewLogger(args)

	filename := strings.TrimSpace(c.String("filename"))
	if filename == "" {
		return nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	//noinspection GoUnhandledErrorResult
	defer f.Close()


	var theparser parser.Parser
	var consumer consumers.Consumer

	app := fx.New(
		consumers.ConsumerService,
		parser.Service,
		extractors.ExifToolService,
		utils.GeoIPService,

		fx.Provide(
			func() *cli.Context { return c },
			func() *arguments.Args { return args },
			func() log15.Logger { return logger },
		),
		fx.Logger(logging.PrintfLogger{Logger: logger}),
		fx.Invoke(func(p parser.Parser, c consumers.Consumer) {
			// bootstrap the application
			theparser = p
			consumer = c
		}),
	)
	done := app.Done()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for range done {
			cancel()
		}
	}()

	startCtx, _ := context.WithTimeout(ctx, app.StartTimeout())
	err = app.Start(startCtx)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("mbox action failed to start: %s", err), 1)
	}
	stopCtx, _ := context.WithTimeout(ctx, app.StopTimeout())
	defer app.Stop(stopCtx)

	g, lctx := errgroup.WithContext(ctx)
	incomings := make(chan *models.IncomingMail)
	features := make(chan *models.FeaturesMail)

	g.Go(func() error {
		theparser.ParseMany(lctx, incomings, features)
		return nil
	})

	g.Go(func() error {
		for {
			select {
			case <-lctx.Done():
				return lctx.Err()
			case feature, ok := <-features:
				if !ok {
					return nil
				}
				err := consumer.Consume(feature)
				if err != nil {
					return err
				}
			}
		}
	})

	g.Go(func() error {
		err := scanMbox(lctx, f, incomings, logger)
		close(incomings)
		if err != nil {
			logger.Warn("Error parsing mail from mbox", "error", err)
		}
		return nil
	})

	_ = g.Wait()

	return nil
}


func scanMbox(ctx context.Context, r io.Reader, incomings chan<- *models.IncomingMail, logger log15.Logger) error {
	scanner := mbox.NewScanner(r)
	for scanner.Next() {
		msg := scanner.Message()
		var data bytes.Buffer
		for k, values := range msg.Header {
			for _, v := range values {
				data.WriteString(k)
				data.WriteString(": ")
				data.WriteString(v)
				data.WriteByte('\n')
			}
		}
		data.WriteByte('\n')
		_, err := io.Copy(&data, msg.Body)
		if err != nil {
			logger.Warn("Error reading mail from mbox", "error", err)
			continue
		}
		incoming := &models.IncomingMail{
			BaseInfos: models.BaseInfos{
				Family:       "mbox",
				TimeReported: time.Now(),
			},
			Data: data.Bytes(),
		}
		select {
		case incomings <- incoming:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return scanner.Err()
}