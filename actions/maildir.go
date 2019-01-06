package actions

import (
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func MaildirAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		err = fmt.Errorf("error validating cli arguments: %s", err)
		return cli.NewExitError(err.Error(), 1)
	}

	logger := logging.NewLogger(args)

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
		defer close(incomings)
		for _, dir := range directories {
			err := readDir(lctx, dir, incomings, logger)
			if err != nil {
				logger.Warn("Error reading maildir", "error", err)
				return err
			}
		}
		return nil
	})

	_ = g.Wait()
	return nil

}

func readDir(ctx context.Context, dir string, incomings chan<- *models.IncomingMail, logger log15.Logger) error {
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
			err := readDir(ctx, name, incomings, logger)
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
		select {
		case incomings <- incoming:
		case <-ctx.Done():
			return ctx.Err()
		}

	}
	return nil
}
