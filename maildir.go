package main

import (
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func MaildirAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		return err
	}
	logger := args.Logging.Build()

	directory := strings.TrimSpace(c.String("directory"))
	if directory == "" {
		return nil
	}
	infos, err := os.Stat(directory)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Stat error for '%s': %s", directory, err), 1)
	}
	if !infos.IsDir() {
		return cli.NewExitError(fmt.Sprintf("'%s' is not a directory", directory), 1)
	}
	d, err := os.Open(directory)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Open error for '%s': %s", directory, err), 1)
	}
	//noinspection GoUnhandledErrorResult
	_ = d.Close()
	directories := make([]string, 0)
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "tmp" {
				return filepath.SkipDir
			}
			if base == "cur" || base == "new" {
				directories = append(directories, path)
				return filepath.SkipDir
			}
		}
		return nil
	})
	logger.Info("Directories to inspect", "names", directories)

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
		}
	}()

	g, ctx := errgroup.WithContext(gctx)

	parser := NewParser(logger)

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, args.NbParsers, logger)
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

	g.Go(func() error {
		defer func() {
			logger.Info("No more messages")
			_ = collector.Close()
		}()
		for _, dir := range directories {
			err := readDir(ctx, dir, collector, logger)
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


func readDir(ctx context.Context, dir string, collector collectors.Collector, logger log15.Logger) error {
	d, err := os.Open(dir)
	if err != nil {
		logger.Warn("Failed to open directory", "name", d, "error", err)
		return nil
	}
	infos, err := d.Readdir(0)
	_ = d.Close()
	if err != nil {
		logger.Warn("Failed to list files in directory", "name", d, "error", err)
	}
	for _, info := range infos {
		name := filepath.Join(dir, info.Name())
		if info.IsDir() {
			err := readDir(ctx, name, collector, logger)
			if err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			logger.Warn("Failed to open file", "name", name, "error", err)
			continue
		}
		content, err := ioutil.ReadAll(f)
		_ = f.Close()
		if err != nil {
			logger.Warn("Failed to read file", "name", name, "error", err)
			continue
		}
		incoming := &models.IncomingMail{
			BaseInfos: models.BaseInfos{
				Family:       "maildir",
				TimeReported: time.Now(),
			},
			Data: content,
		}
		err = collector.PushCtx(ctx, incoming)
		if err != nil {
			return err
		}
	}
	return nil
}