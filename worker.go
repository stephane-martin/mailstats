package main

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/pkg/errors"
	"github.com/schollz/pake"
	"github.com/stephane-martin/mailstats/sbox"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

type WorkerClient struct {
	HTTP       *http.Client
	secret     *memguard.LockedBuffer
	sessionKey *memguard.LockedBuffer
	uid        ulid.ULID
	logger     log15.Logger
	requestID  uint64
}

func NewWorker(secret *memguard.LockedBuffer, logger log15.Logger) *WorkerClient {
	return &WorkerClient{HTTP: utils.NewHTTPClient(time.Minute), secret: secret, uid: utils.NewULID(), logger: logger, requestID: 0}
}

func (w *WorkerClient) ping(ctx context.Context) bool {
	req, _ := http.NewRequest("GET", "http://127.0.0.1:8080/status", nil)
	req = req.WithContext(ctx)
	resp, err := w.HTTP.Do(req)
	select {
	case <-ctx.Done():
		return true
	default:
	}
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	return true
}

func (w *WorkerClient) submit(ctx context.Context, features *models.FeaturesMail) error {
	body, err := json.Marshal(features)
	if err != nil {
		return err
	}
	enc, err := sbox.Encrypt(body, w.sessionKey)
	if err != nil {
		return err
	}

	httpreq, err := http.NewRequest(
		"POST",
		fmt.Sprintf("http://127.0.0.1:8080/worker/submit/%s", w.uid.String()),
		bytes.NewReader(enc),
	)
	if err != nil {
		return err
	}
	httpreq.Header.Set("Content-Type", "application/octet-stream")
	httpreq = httpreq.WithContext(ctx)

	resp, err := w.HTTP.Do(httpreq)

	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("wrong response status code")
	}
	return nil
}

func (w *WorkerClient) getWork(ctx context.Context) (*models.IncomingMail, error) {
	resp, err := w.doRequest(ctx, "work")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("wrong response status code")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	dec, err := sbox.Decrypt(body, w.sessionKey)
	if err != nil {
		return nil, err
	}
	var m models.IncomingMail
	_, err = m.UnmarshalMsg(dec)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (w *WorkerClient) bye() error {
	resp, err := w.doRequest(context.Background(), "bye")
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("wrong response status code")
	}
	return nil
}

func (w *WorkerClient) ACK(ctx context.Context, uid ulid.ULID) error {
	u := fmt.Sprintf("http://127.0.0.1:8080/worker/ack/%s", w.uid.String())
	req := &ackRequest{UID: uid.String()}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	enc, err := sbox.Encrypt(body, w.sessionKey)
	if err != nil {
		return err
	}
	httpreq, err := http.NewRequest("POST", u, bytes.NewReader(enc))
	if err != nil {
		return err
	}
	httpreq.Header.Set("Content-Type", "application/octet-stream")
	httpreq = httpreq.WithContext(ctx)
	_, err = w.HTTP.Do(httpreq)
	return err
}

func (w *WorkerClient) doRequest(ctx context.Context, kind string, features ...*models.FeaturesMail) (*http.Response, error) {
	w.requestID++

	var req interface{}
	var u string
	switch kind {
	case "work":
		req = &workRequest{RequestID: w.requestID}
		u = fmt.Sprintf("http://127.0.0.1:8080/worker/work/%s", w.uid.String())
	case "bye":
		req = &byeRequest{RequestID: w.requestID}
		u = fmt.Sprintf("http://127.0.0.1:8080/worker/bye/%s", w.uid.String())
	default:
		return nil, errors.New("wrong kind of request")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	enc, err := sbox.Encrypt(body, w.sessionKey)
	if err != nil {
		return nil, err
	}
	httpreq, err := http.NewRequest("POST", u, bytes.NewReader(enc))
	if err != nil {
		return nil, err
	}
	httpreq.Header.Set("Content-Type", "application/octet-stream")
	httpreq = httpreq.WithContext(ctx)
	resp, err := w.HTTP.Do(httpreq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (w *WorkerClient) Auth(ctx context.Context) error {
	curve := elliptic.P521()
	p, err := pake.Init(w.secret.Buffer(), 0, curve)
	if err != nil {
		return err
	}

	r := &initRequest{Pake: base64.StdEncoding.EncodeToString(p.Bytes())}
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://127.0.0.1:8080/worker/init/%s", w.uid.String()),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	resp, err := w.HTTP.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("init response status is not 200: %d", resp.StatusCode)
	}
	var ir initResponse
	err = json.NewDecoder(resp.Body).Decode(&ir)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	hk, err := base64.StdEncoding.DecodeString(ir.HK)
	if err != nil {
		return err
	}

	chanUpdate := make(chan struct{})
	go func() {
		err = p.Update(hk)
		close(chanUpdate)
	}()
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-chanUpdate:
	}

	if err != nil {
		return err
	}

	r2 := &authRequest{HK: base64.StdEncoding.EncodeToString(p.Bytes())}
	body, err = json.Marshal(r2)
	if err != nil {
		return err
	}
	req, _ = http.NewRequest(
		"POST",
		fmt.Sprintf("http://127.0.0.1:8080/worker/auth/%s", w.uid.String()),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err = w.HTTP.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("auth response status is not 200: %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	skey, err := p.SessionKey()
	if err != nil {
		return err
	}
	w.sessionKey, err = memguard.NewImmutableFromBytes(skey)
	return err
}

func WorkerAction(c *cli.Context) error {
	var logArgs arguments.LoggingArgs
	logArgs.Populate(c)
	err := logArgs.Verify()
	if err != nil {
		return err
	}
	logger := logArgs.Build()
	nbParsers := c.GlobalInt("nbparsers")

	if nbParsers == 0 {
		return nil
	}
	if nbParsers < 0 {
		nbParsers = runtime.NumCPU()
	}

	sec := strings.TrimSpace(c.GlobalString("secret"))
	if len(sec) == 0 {
		return cli.NewExitError("secret is not set", 2)
	}
	secret, err := memguard.NewImmutableFromBytes([]byte(sec))
	if err != nil {
		return cli.NewExitError("Failed to create memguard", 2)
	}

	gctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for range sigChan {
			cancel()
		}
	}()

W:
	for {
		err = worker(gctx, secret, nbParsers, logger)

		if err == nil || err == context.Canceled {
			break W
		}

		if strings.Contains(err.Error(), "connection refused") || utils.IsTemp(err) || utils.IsTimeout(err) || err == io.EOF {
			select {
			case <-gctx.Done():
				break W
			case <-time.After(5 * time.Second):
				continue W
			}
		}
		if e, ok := err.(*url.Error); ok {
			logger.Info("URL error", "op", e.Op, "url", e.URL, "error", e.Err)
			if e.Err == io.EOF || utils.IsTemp(e.Err) || utils.IsTimeout(e.Err) {
				select {
				case <-gctx.Done():
					break W
				case <-time.After(5 * time.Second):
					continue W
				}
			}
		}
		break W
	}
	logger.Info("Worker stopped", "error", err)

	return nil
}

func worker(gctx context.Context, secret *memguard.LockedBuffer, nbParsers int, logger log15.Logger) error {
	g, ctx := errgroup.WithContext(gctx)

	worker := NewWorker(secret, logger)

Ping:
	for {
		ok := worker.ping(ctx)
		if ok {
			break Ping
		}
		logger.Info("Server not reachable")
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-time.After(5 * time.Second):
		}
	}
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	logger.Info("Starting authentication")
	err := worker.Auth(ctx)
	if err != nil {
		return err
	}
	logger.Info("Worker is authenticated")

	parser := NewParser(logger)

	ch := make(chan *models.IncomingMail)

	g.Go(func() error {
		defer close(ch)
		for {
			m, err := worker.getWork(ctx)
			if err == nil {
				logger.Debug("Received work", "uid", ulid.ULID(m.UID).String())
				ch <- m
			} else {
				if err == context.Canceled {
					return context.Canceled
				}
				if e, ok := err.(*url.Error); ok {
					if e.Err == context.Canceled {
						return context.Canceled
					}
				}
				logger.Info("Error getting work", "error", err)
				if !utils.IsTimeout(err) && !utils.IsTemp(err) {
					return err
				}
				time.Sleep(time.Second)
			}
		}
	})

	for i := 0; i < nbParsers; i++ {
		g.Go(func() error {
			for m := range ch {
				features, parseErr := parser.Parse(m)

				g.Go(func() error {
					for {
						var err error
						if parseErr != nil {
							logger.Info("Worker failed to parse message", "error", parseErr)
							err = worker.ACK(ctx, m.UID)
						} else {
							err = worker.submit(ctx, features)
						}
						if err == nil || err == context.Canceled {
							return err
						}
						if e, ok := err.(*url.Error); ok {
							if e.Err == context.Canceled {
								return context.Canceled
							}
						}
						logger.Warn("Failed to upload results", "error", err)
						if !utils.IsTemp(err) && !utils.IsTimeout(err) {
							return err
						}
						time.Sleep(time.Second)
					}

				})

			}
			return nil
		})
	}

	err = g.Wait()
	_ = parser.Close()

	if err != nil && err != context.Canceled {
		logger.Info("Worker returned", "error", err)
	}

	return err
}
