package main

import (
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net"
	"net/http"
	"strings"
)

type HTTPArgs struct {
	ListenAddr string
	ListenPort int
}

func (args HTTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The HTTP listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The HTTP listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The HTTP listen address is invalid")
	return v.GetError()
}

func (args *HTTPArgs) Populate(c *cli.Context) *HTTPArgs {
	if args == nil {
		args = new(HTTPArgs)
	}
	args.ListenPort = c.GlobalInt("http-port")
	args.ListenAddr = strings.TrimSpace(c.GlobalString("http-addr"))
	return args
}

func StartHTTP(ctx context.Context, args HTTPArgs, logger log15.Logger) error {
	if args.ListenPort <= 0 {
		return nil
	}
	if args.ListenAddr == "" {
		args.ListenAddr = "127.0.0.1"
	}

	muxer := http.NewServeMux()

	muxer.Handle(
		"/metrics",
		promhttp.HandlerFor(
			M().Registry,
			promhttp.HandlerOpts{
				DisableCompression:  true,
				ErrorLog:            adaptPromLogger(logger),
				ErrorHandling:       promhttp.HTTPErrorOnError,
				MaxRequestsInFlight: -1,
				Timeout:             -1,
			},
		),
	)

	muxer.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	svc := &http.Server{
		Addr:    net.JoinHostPort(args.ListenAddr, fmt.Sprintf("%d", args.ListenPort)),
		Handler: muxer,
	}

	go func() {
		<-ctx.Done()
		_ = svc.Close()
		logger.Info("HTTP service closed")
	}()

	logger.Info("Starting HTTP service")
	err := svc.ListenAndServe()
	if err != nil {
		logger.Info("HTTP service error", "error", err)
	}
	return err

}
