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
	"log"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gin-gonic/gin"
	"github.com/go-gomail/gomail"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/schollz/pake"
	"github.com/stephane-martin/mailstats/sbox"
	"github.com/storozhukBM/verifier"
	"github.com/uber-go/atomic"
	"github.com/urfave/cli"
	"golang.org/x/text/encoding/htmlindex"
)

var pakeRecipients *PakeRecipients
var pakeSessionKeys *SessionKeyStore
var increments *CurrentIncrements

func init() {
	pakeRecipients = NewPakeRecipients()
	pakeSessionKeys = NewSessionKeyStore()
	increments = NewIncrements()
}

type CurrentIncrements struct {
	m *sync.Map
}

func NewIncrements() *CurrentIncrements {
	return &CurrentIncrements{m: new(sync.Map)}
}

func (i *CurrentIncrements) NewWorker(workerID ulid.ULID) {
	i.m.Store(workerID, atomic.NewUint64(0))
}

func (i *CurrentIncrements) Check(workerID ulid.ULID, increment uint64) error {
	inc, ok := i.m.Load(workerID)
	if !ok {
		return errors.New("unknown worker")
	}
	if !(inc.(*atomic.Uint64).Inc() == increment) {
		// TODO: too brutal
		i.Erase(workerID)
		return errors.New("wrong increment")
	}
	return nil
}

func (i *CurrentIncrements) Erase(workerID ulid.ULID) {
	i.m.Delete(workerID)
}

type SessionKeyStore struct {
	m *sync.Map
}

func NewSessionKeyStore() *SessionKeyStore {
	return &SessionKeyStore{m: new(sync.Map)}
}

func (r *SessionKeyStore) Has(workerID ulid.ULID) bool {
	_, ok := r.m.Load(workerID)
	return ok
}

func (r *SessionKeyStore) Put(workerID ulid.ULID, key []byte) error {
	l, err := memguard.NewImmutableFromBytes(key)
	if err != nil {
		return err
	}
	_, loaded := r.m.LoadOrStore(workerID, l)
	if loaded {
		l.Destroy()
		return errors.New("worker already initialized")
	}
	return nil
}

func (r *SessionKeyStore) Get(workerID ulid.ULID) (key *memguard.LockedBuffer, err error) {
	rec, ok := r.m.Load(workerID)
	if !ok {
		return nil, errors.New("unknown worker")
	}
	return rec.(*memguard.LockedBuffer), nil
}

func (r *SessionKeyStore) Erase(workerID ulid.ULID) {
	r.m.Delete(workerID)
}

func NewRecipient(secret *memguard.LockedBuffer) (*pake.Pake, error) {
	curve := elliptic.P521()
	recipient, err := pake.Init(secret.Buffer(), 1, curve)
	if err != nil {
		return nil, err
	}
	return recipient, nil
}

type PakeRecipients struct {
	m *sync.Map
}

func NewPakeRecipients() *PakeRecipients {
	return &PakeRecipients{m: new(sync.Map)}
}

func (r *PakeRecipients) Has(workerID ulid.ULID) bool {
	_, ok := r.m.Load(workerID)
	return ok
}

func (r *PakeRecipients) Put(workerID ulid.ULID, recipient *pake.Pake) error {
	_, loaded := r.m.LoadOrStore(workerID, recipient)
	if loaded {
		return errors.New("worker already initialized")
	}
	return nil
}

func (r *PakeRecipients) Get(workerID ulid.ULID) (recipient *pake.Pake, err error) {
	rec, ok := r.m.Load(workerID)
	if !ok {
		return nil, errors.New("unknown worker")
	}
	return rec.(*pake.Pake), nil
}

func (r *PakeRecipients) Erase(workerID ulid.ULID) {
	r.m.Delete(workerID)
}

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
		//noinspection GoAssignmentToReceiver
		args = new(HTTPArgs)
	}
	args.ListenPort = c.GlobalInt("http-port")
	args.ListenAddr = strings.TrimSpace(c.GlobalString("http-addr"))
	return args
}

type initRequest struct {
	Pake string `json:"pake"`
}

type initResponse struct {
	HK string `json:"hk"`
}

type authRequest struct {
	HK string `json:"hk"`
}

type workRequest struct {
	RequestID uint64 `json:"request_id"`
}

type byeRequest struct {
	RequestID uint64 `json:"request_id"`
}

type submitRequest struct {
	RequestID uint64 `json:"request_id"`
	Features  *FeaturesMail
}

func prepare(obj interface{}, c *gin.Context) (ulid.ULID, error) {
	workerID, err := ulid.Parse(c.Param("worker"))
	if err != nil {
		return workerID, fmt.Errorf("failed to parse worker ID: %s", err)
	}
	key, err := pakeSessionKeys.Get(workerID)
	if err != nil {
		return workerID, fmt.Errorf("failed to get session key: %s", err)
	}
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return workerID, fmt.Errorf("failed to read body: %s", err)
	}
	dec, err := sbox.Decrypt(body, key)
	if err != nil {
		return workerID, fmt.Errorf("failed to decrypt body: %s", err)
	}
	err = json.Unmarshal(dec, obj)
	if err != nil {
		return workerID, fmt.Errorf("failed to unmarshal body: %s", err)
	}
	if _, ok := reflect.ValueOf(obj).Elem().Type().FieldByName("RequestID"); ok {
		increment := reflect.ValueOf(obj).Elem().FieldByName("RequestID").Uint()
		err = increments.Check(workerID, increment)
		if err != nil {
			return workerID, fmt.Errorf("increment check failed: %s", err)
		}
	}
	return workerID, nil
}

type log15Writer struct {
	logger log15.Logger
}

func (w *log15Writer) Write(b []byte) (int, error) {
	w.logger.Info(string(b))
	return len(b), nil
}

func StartHTTP(ctx context.Context, args HTTPArgs, secret *memguard.LockedBuffer, collector Collector, consumer Consumer, logger log15.Logger) error {
	if args.ListenPort <= 0 {
		return nil
	}
	if args.ListenAddr == "" {
		args.ListenAddr = "127.0.0.1"
	}
	gin.SetMode(GinMode)
	gin.DisableConsoleColor()
	wr := &log15Writer{logger: logger}
	gin.DefaultWriter = wr
	gin.DefaultErrorWriter = wr
	log.SetOutput(wr)
	router := gin.Default()

	router.Any("/metrics", gin.WrapH(
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
	))

	router.GET("/status", func(c *gin.Context) {
		c.Status(200)
	})

	if secret != nil {
		router.POST("/worker/init/:worker", func(c *gin.Context) {
			workerID, err := ulid.Parse(c.Param("worker"))
			if err != nil {
				logger.Warn("Failed to parse worker ID", "error", err)
				c.Status(http.StatusBadRequest)
				return
			}
			var pInit initRequest
			_ = c.BindJSON(&pInit)
			logger.Debug("init request", "worker", workerID.String())
			if pakeSessionKeys.Has(workerID) {
				logger.Warn("Worker is already authenticated")
				c.Status(http.StatusBadRequest)
				return
			}
			if pakeRecipients.Has(workerID) {
				logger.Warn("Worker is already initialized")
				c.Status(http.StatusBadRequest)
				return
			}
			p, err := base64.StdEncoding.DecodeString(pInit.Pake)
			if err != nil {
				logger.Warn("Failed to base64 decode pake init request", "error", err)
				c.Status(http.StatusBadRequest)
				return
			}
			if secret == nil {
				logger.Warn("Got a pake init request, but secret is not set")
				c.Status(http.StatusBadRequest)
				return
			}
			recipient, err := NewRecipient(secret)
			if err != nil {
				logger.Warn("Failed to initialize pake recipient", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			err = recipient.Update(p)
			if err != nil {
				logger.Warn("Failed to update pake recipient", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			err = pakeRecipients.Put(workerID, recipient)
			if err != nil {
				logger.Warn("Failed to store new PAKE recipient", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			c.JSON(200, initResponse{HK: base64.StdEncoding.EncodeToString(recipient.Bytes())})
		})

		router.POST("/worker/auth/:worker", func(c *gin.Context) {
			workerID, err := ulid.Parse(c.Param("worker"))
			if err != nil {
				logger.Warn("Failed to parse worker ID", "error", err)
				c.Status(http.StatusBadRequest)
				return
			}
			logger.Debug("auth request", "worker", workerID.String())
			if pakeSessionKeys.Has(workerID) {
				logger.Warn("Worker is already authenticated")
				c.Status(http.StatusBadRequest)
				return
			}
			recipient, err := pakeRecipients.Get(workerID)
			if err != nil {
				logger.Warn("Worker is not initialized", "error", err)
				c.Status(http.StatusBadRequest)
				return
			}

			var pAuth authRequest
			_ = c.BindJSON(&pAuth)
			hk, err := base64.StdEncoding.DecodeString(pAuth.HK)
			if err != nil {
				logger.Warn("Failed to base64 decode work HK", "error", err)
				c.Status(http.StatusBadRequest)
				return
			}

			err = recipient.Update(hk)
			if err != nil {
				logger.Warn("Failed to update recipient after auth request", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			skey, err := recipient.SessionKey()
			if err != nil {
				logger.Warn("Failed to retrieve session key", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			err = pakeSessionKeys.Put(workerID, skey)
			if err != nil {
				logger.Warn("Failed to store new session key", "error", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			pakeRecipients.Erase(workerID)
			increments.NewWorker(workerID)
		})

		router.POST("/worker/bye/:worker", func(c *gin.Context) {
			var obj byeRequest
			workerID, err := prepare(&obj, c)
			if err != nil {
				logger.Warn("Error decoding worker request", "error", err)
				return
			}
			increments.Erase(workerID)
			pakeSessionKeys.Erase(workerID)
		})

		router.POST("/worker/work/:worker", func(c *gin.Context) {
			var obj workRequest
			workerID, err := prepare(&obj, c)
			if err != nil {
				logger.Warn("Error decoding worker request", "error", err)
				return
			}
			work, err := collector.PullCtx(c.Request.Context())
			if err == nil {
				j, err := work.MarshalMsg(nil)
				if err != nil {
					c.Status(500)
					return
				}
				key, err := pakeSessionKeys.Get(workerID)
				if err != nil {
					c.Status(500)
					return
				}
				enc, err := sbox.Encrypt(j, key)
				if err != nil {
					c.Status(500)
					return
				}
				c.Data(200, "application/octet-stream", enc)
				return
			}
			if err == context.Canceled {
				logger.Debug("Worker is gone")
				return
			}
			logger.Warn("Error getting some work", "error", err)
			c.Status(500)
		})

		router.POST("/worker/submit/:worker", func(c *gin.Context) {
			var features FeaturesMail
			_, err := prepare(&features, c)
			if err != nil {
				logger.Warn("Error decoding worker request", "error", err)
				return
			}
			go func() {
				consumer.Consume(&features)
			}()
		})
	}

	router.POST("/messages.mime", func(c *gin.Context) {
		now := time.Now()

		ct := c.ContentType()
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			logger.Warn("Error parsing media type", "error", err)
			c.Status(400)
			return
		}

		charset := strings.TrimSpace(params["charset"])
		if charset == "" {
			charset = "utf-8"
		}
		encoding, err := htmlindex.Get(charset)
		if err != nil {
			logger.Warn("Failed to get decoder", "charset", charset)
			c.Status(500)
			return
		}
		decoder := encoding.NewDecoder()
		decode := func(s string) string {
			res, err := decoder.String(s)
			if err != nil {
				return s
			}
			return res
		}

		var message []byte
		fh, err := c.FormFile("message")
		if err == http.ErrMissingFile {
			message = []byte(decode(c.PostForm("message")))
		} else if err != nil {
			logger.Warn("Failed to get message part", "error", err)
			c.Status(500)
			return
		} else {
			f, err := fh.Open()
			if err != nil {
				logger.Warn("Failed to get message part", "error", err)
				c.Status(500)
				return
			}
			//noinspection GoUnhandledErrorResult
			defer f.Close()
			b, err := ioutil.ReadAll(f)
			if err != nil {
				logger.Warn("Failed to read message part", "error", err)
				c.Status(500)
				return
			}
			message = b
		}
		message = bytes.TrimSpace(message)
		if len(message) == 0 {
			logger.Warn("Empty message")
			c.Status(400)
			return
		}

		var sender *mail.Address
		from := strings.TrimSpace(decode(c.PostForm("from")))
		if len(from) > 0 {
			s, err := mail.ParseAddress(from)
			if err == nil {
				sender = s
			}
		}

		recipients := make([]*mail.Address, 0)
		for _, recipient := range c.PostFormArray("to") {
			recipientAddr, err := mail.ParseAddress(decode(recipient))
			if err == nil {
				recipients = append(recipients, recipientAddr)
			}
		}

		parsed, err := mail.ReadMessage(bytes.NewReader(message))
		if err != nil {
			logger.Warn("ReadMessage() error", "error", err)
			c.Status(500)
			return
		}

		if sender == nil {
			from := strings.TrimSpace(parsed.Header.Get("From"))
			if len(from) > 0 {
				s, err := mail.ParseAddress(from)
				if err == nil {
					sender = s
				}
			}
		}

		if len(recipients) == 0 {
			to := parsed.Header["To"]
			for _, recipient := range to {
				r, err := mail.ParseAddress(recipient)
				if err == nil {
					recipients = append(recipients, r)
				}
			}
		}

		infos := new(IncomingMail)
		infos.Data = []byte(message)
		infos.TimeReported = now
		infos.UID = NewULID()
		if sender != nil {
			infos.MailFrom = sender.Address
		}
		for _, recipient := range recipients {
			infos.RcptTo = append(infos.RcptTo, recipient.Address)
		}
		infos.Addr = c.ClientIP()
		infos.Family = "http"
		infos.Port = args.ListenPort
		err = collector.PushCtx(ctx, infos)
		if err != nil {
			logger.Error("Error pushing HTTP message to collector", "error", err)
			c.Status(500)
			return
		}

		if len(c.Accepted) == 0 {
			return
		}
		if c.NegotiateFormat("application/json") == "" {
			return
		}
		parser := NewParser(logger)
		defer func() { _ = parser.Close() }()
		features, err := parser.Parse(infos)
		if err != nil {
			logger.Warn("Error calculating features", "error", err)
			c.Status(500)
			return
		}
		c.JSON(200, features)
	})

	router.POST("/messages", func(c *gin.Context) {
		// cf https://documentation.mailgun.com/en/latest/api-sending
		now := time.Now()
		_, params, err := mime.ParseMediaType(c.ContentType())
		charset := strings.TrimSpace(params["charset"])
		if charset == "" {
			charset = "utf-8"
		}
		encoding, err := htmlindex.Get(charset)
		if err != nil {
			logger.Warn("Failed to get decoder", "charset", charset)
			c.Status(500)
			return
		}
		decoder := encoding.NewDecoder()
		decode := func(s string) string {
			res, err := decoder.String(s)
			if err != nil {
				return s
			}
			return res
		}

		message := gomail.NewMessage(
			gomail.SetCharset("utf8"),
			gomail.SetEncoding(gomail.Unencoded),
		)
		message.SetHeader("Date", message.FormatDate(now))

		// from, to, cc, bcc, subject, text, html, attachment
		from := strings.TrimSpace(decode(c.PostForm("from")))
		if len(from) > 0 {
			sender, err := mail.ParseAddress(from)
			if err == nil {
				from = sender.Address
				message.SetAddressHeader("From", sender.Address, sender.Name)
			} else {
				from = ""
			}
		}

		to := make([]*mail.Address, 0)
		for _, recipient := range c.PostFormArray("to") {
			recipientAddr, err := mail.ParseAddress(decode(recipient))
			if err == nil {
				to = append(to, recipientAddr)
			}
		}
		encodedTo := make([]string, 0)
		for _, addr := range to {
			encodedTo = append(encodedTo, message.FormatAddress(addr.Address, addr.Name))
		}
		if len(encodedTo) > 0 {
			message.SetHeader("To", encodedTo...)
		}

		cc := make([]string, 0)
		for _, recipient := range c.PostFormArray("cc") {
			r, err := mail.ParseAddress(decode(recipient))
			if err == nil {
				cc = append(cc, message.FormatAddress(r.Address, r.Name))
			}
		}
		if len(cc) > 0 {
			message.SetHeader("Cc", cc...)
		}

		bcc := make([]string, 0)
		for _, recipient := range c.PostFormArray("bcc") {
			r, err := mail.ParseAddress(decode(recipient))
			if err == nil {
				bcc = append(bcc, message.FormatAddress(r.Address, r.Name))
			}
		}
		if len(bcc) > 0 {
			message.SetHeader("Bcc", bcc...)
		}

		subject := decode(c.PostForm("subject"))
		if len(subject) > 0 {
			message.SetHeader("Subject", subject)
		}

		text := strings.TrimSpace(decode(c.PostForm("text")))
		html := strings.TrimSpace(decode(c.PostForm("html")))

		if len(text) > 0 && len(html) > 0 {
			message.SetBody("text/plain", text)
			message.AddAlternative("text/html", html)
		} else if len(text) > 0 {
			message.SetBody("text/plain", text)
		} else if len(html) > 0 {
			message.SetBody("text/html", html)
		}

		forms, err := c.MultipartForm()
		if err != nil {
			logger.Warn("Error parsing multipart", "error", err)
			c.Status(500)
			return
		}

		if forms != nil {
			for _, fheader := range forms.File["attachment"] {
				message.Attach(fheader.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
					f, err := fheader.Open()
					if err != nil {
						return err
					}
					defer func() { _ = f.Close() }()
					_, err = io.Copy(w, f)
					return err
				}))
			}
		}

		var b bytes.Buffer
		_, err = message.WriteTo(&b)
		if err != nil {
			logger.Warn("Error marshalling message to MIME", "error", err)
			c.Status(500)
			return
		}
		infos := new(IncomingMail)
		infos.Data = b.Bytes()
		infos.TimeReported = now
		infos.UID = NewULID()
		if len(from) > 0 {
			infos.MailFrom = from
		}
		for _, addr := range to {
			infos.RcptTo = append(infos.RcptTo, addr.Address)
		}
		infos.Addr = c.ClientIP()
		infos.Family = "http"
		infos.Port = args.ListenPort
		err = collector.PushCtx(ctx, infos)
		if err != nil {
			logger.Error("Error pushing HTTP message to collector", "error", err)
			c.Status(500)
			return
		}

		if len(c.Accepted) == 0 {
			return
		}

		if c.NegotiateFormat("application/json") == "" {
			return
		}

		parser := NewParser(logger)
		defer func() { _ = parser.Close() }()
		features, err := parser.Parse(infos)
		if err != nil {
			logger.Warn("Error calculating features", "error", err)
			c.Status(500)
			return
		}
		c.JSON(200, features)

	})

	svc := &http.Server{
		Addr:    net.JoinHostPort(args.ListenAddr, fmt.Sprintf("%d", args.ListenPort)),
		Handler: router,
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
