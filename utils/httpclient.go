package utils

import (
	"net"
	"net/http"
	"time"
)

func NewHTTPClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		DisableCompression: true,
		MaxIdleConns: 16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout: 30 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
	}
	return &http.Client{Transport: tr}
}


