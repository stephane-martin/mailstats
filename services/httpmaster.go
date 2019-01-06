package services

import (
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/awnumar/memguard"
	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/schollz/pake"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/sbox"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/uber-go/atomic"
	"go.uber.org/fx"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"
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

type ackRequest struct {
	UID string `json:"uid"`
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


type HTTPMasterEngine *gin.Engine

type HTTPMasterServer struct {
	*http.Server
	logger log15.Logger
	addr string
	port int
}


func NewHTTPMasterEngine(secret *memguard.LockedBuffer, collector collectors.Collector, consumer consumers.Consumer, logger log15.Logger) HTTPMasterEngine {
	router := gin.Default()
	workerTimes := &sync.Map{}

	router.GET("/status", func(c *gin.Context) {
		c.Status(200)
	})

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
			workerTimes.Store(ulid.ULID(work.UID), time.Now())
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
		features := new(models.FeaturesMail)
		_, err := prepare(features, c)
		if err != nil {
			logger.Warn("Error decoding worker request", "error", err)
			return
		}
		uid := ulid.MustParse(features.UID)
		if start, ok := workerTimes.Load(uid); ok {
			metrics.M().ParsingDuration.Observe(time.Now().Sub(start.(time.Time)).Seconds())
			workerTimes.Delete(uid)
		}
		collector.ACK(uid)
		go func() {
			err := consumer.Consume(features)
			if err != nil {
				logger.Warn("Failed to consume parsing results", "error", err)
			} else {
				logger.Debug("Parsing results sent to consumer")
			}
		}()
	})

	router.POST("/worker/ack/:worker", func(c *gin.Context) {
		var obj ackRequest
		_, err := prepare(&obj, c)
		if err != nil {
			logger.Warn("Error decoding ACK request", "error", err)
			return
		}
		uid, err := ulid.Parse(obj.UID)
		if err == nil {
			collector.ACK(uid)
		}
	})

	return router
}

func (s HTTPMasterServer) Name() string { return "HTTPMasterServer" }

func (s HTTPMasterServer) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.addr, fmt.Sprintf("%d", s.port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = s.Server.Close()
	}()
	s.logger.Info("Starting HTTP Master service")
	return s.Server.Serve(listener)
}

func NewHTTPMasterServer(args *arguments.Args, collector collectors.Collector, consumer consumers.Consumer, logger log15.Logger) *HTTPMasterServer {
	var engine *gin.Engine = NewHTTPMasterEngine(args.Secret, collector, consumer, logger)

	server := &http.Server{
		Handler: engine,
	}

	return &HTTPMasterServer{
		logger: logger,
		Server: server,
		addr: args.HTTP.ListenAddrMaster,
		port: args.HTTP.ListenPortMaster,
	}
}

var HTTPMasterService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, collector collectors.Collector, consumer consumers.Consumer, logger log15.Logger) *HTTPMasterServer {
	if args.Secret == nil {
		return nil
	}
	s := NewHTTPMasterServer(args, collector, consumer, logger)
	utils.Append(lc, s, logger)
	return s
})
