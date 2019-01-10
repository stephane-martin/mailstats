package utils

import (
	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"net"
	"net/http"
	"time"
)

var initialPause = time.Second
var maxPause = 30 * time.Second
var jitter = 100 * time.Millisecond

func newHDClient(timeout time.Duration, retries int, tr http.RoundTripper) *httpclient.Client {
	retrier := heimdall.NewRetrier(
		heimdall.NewExponentialBackoff(initialPause, maxPause, 2, jitter),
	)

	return httpclient.NewClient(
		httpclient.WithHTTPClient(&http.Client{Transport: tr, Timeout: timeout}),
		httpclient.WithRetrier(retrier),
		httpclient.WithRetryCount(retries),
	)
}

func newTransport(maxConns int) http.RoundTripper {
	return &http.Transport{
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
}

func NewHTTPCacheClient(timeout time.Duration, maxConns int, basePath string, retries int) *httpclient.Client {
	tr := httpcache.NewTransport(diskcache.New(basePath))
	tr.Transport = newTransport(maxConns)

	if retries > 0 {
		return newHDClient(timeout, retries, tr)
	}

	return httpclient.NewClient(
		httpclient.WithHTTPClient(&http.Client{Transport: tr, Timeout: timeout}),
	)

}

func NewHTTPClient(timeout time.Duration, maxConns int, retries int) *httpclient.Client {
	tr := newTransport(maxConns)

	if retries > 0 {
		if retries > 0 {
			return newHDClient(timeout, retries, tr)
		}
	}

	return httpclient.NewClient(
		httpclient.WithHTTPClient(&http.Client{Transport: tr, Timeout: timeout}),
	)

}
