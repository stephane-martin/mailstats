package utils

import (
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"
	"net"
	"net/http"
	"time"
)

func NewHTTPClient(timeout time.Duration, maxConns int, retries int) *httpclient.Client {
	tr := &http.Transport{
		MaxConnsPerHost:       maxConns,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		Proxy:                 http.ProxyFromEnvironment,
		DisableCompression:    true,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
	}

	if retries > 0 {
		initialPause := time.Second
		maxPause := 30 * time.Second
		jitter := 100 * time.Millisecond
		backoff := heimdall.NewExponentialBackoff(initialPause, maxPause, 2, jitter)
		retrier := heimdall.NewRetrier(backoff)

		return httpclient.NewClient(
			httpclient.WithHTTPClient(&http.Client{Transport: tr, Timeout: timeout}),
			httpclient.WithRetrier(retrier),
			httpclient.WithRetryCount(retries),
		)
	}

	return httpclient.NewClient(
		httpclient.WithHTTPClient(&http.Client{Transport: tr, Timeout: timeout}),
	)

}
