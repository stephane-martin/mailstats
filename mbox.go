package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/blabber/mbox"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

func MBoxAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		return err
	}
	logger := args.Logging.Build()

	collector, err := collectors.NewChanCollector(args.Collector.CollectorSize, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}

	forwarder := forwarders.DummyForwarder{}

	consumer, err := consumers.MakeConsumer(*args)
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
		}
	}()

	g, ctx := errgroup.WithContext(gctx)

	parser := NewParser(logger)

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, args.NbParsers, logger)
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

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
	scanner := mbox.NewScanner(f)

	g.Go(func() error {
		defer func() {
			_ = collector.Close()
		}()
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
			err = collector.PushCtx(ctx, incoming)
			if err != nil {
				return err
			}
		}
		err := scanner.Err()
		if err != nil {
			logger.Warn("Error parsing mail from mbox", "error", err)
			return err
		}
		logger.Info("No more messages")
		return nil
	})

	err = g.Wait()
	if err != nil && err != context.Canceled {
		logger.Warn("goroutine group error", "error", err)
	}

	return nil
}
