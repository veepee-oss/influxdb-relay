package relay

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"

	"net"
	"net/http"

	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/toni-moreno/influxdb-srelay/config"
)

// HTTP is a relay for HTTP influxdb writes
type HTTP struct {
	cfg    *config.HTTPConfig
	schema string

	cert string
	rp   string

	pingResponseCode    int
	pingResponseHeaders map[string]string

	closing int64
	l       net.Listener

	Endpoints []*HTTPEndPoint

	start  time.Time
	log    *zerolog.Logger
	acclog *zerolog.Logger

	rateLimiter *rate.Limiter
}

// httpError writes an error to the client in a standard format.
func (h *HTTP) httpError(w http.ResponseWriter, errmsg string, code int) {
	// Default implementation if the response writer hasn't been replaced
	// with our special response writer type.
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(errmsg)
	w.Write(b)
}

type relayHandlerFunc func(h *HTTP, w http.ResponseWriter, r *http.Request, start time.Time)
type relayMiddleware func(h *HTTP, handlerFunc relayHandlerFunc) relayHandlerFunc

// Default HTTP settings and a few constants
const (
	DefaultHTTPPingResponse = http.StatusNoContent
	DefaultHTTPTimeout      = 10 * time.Second
	DefaultMaxDelayInterval = 10 * time.Second
	DefaultBatchSizeKB      = 512

	KB = 1024
	MB = 1024 * KB
)

var (
	handlers = map[string]relayHandlerFunc{
		"/ping":        (*HTTP).handlePing,
		"/status":      (*HTTP).handleStatus,
		"/admin":       (*HTTP).handleAdmin,
		"/admin/flush": (*HTTP).handleFlush,
		"/health":      (*HTTP).handleHealth,
	}

	middlewares = []relayMiddleware{
		(*HTTP).bodyMiddleWare,
		(*HTTP).queryMiddleWare,
		(*HTTP).logMiddleWare,
		(*HTTP).rateMiddleware,
	}
)

// NewHTTP creates a new HTTP relay
// This relay will most likely be tied to a RelayService
// and manage a set of HTTPBackends
func NewHTTP(cfg *config.HTTPConfig) (Relay, error) {
	h := new(HTTP)
	h.cfg = cfg

	//Log output

	h.log = GetConsoleLogFormated(cfg.LogFile, cfg.LogLevel)
	//AccessLog Output

	h.acclog = GetConsoleLogFormated(cfg.AccessLog, "debug")

	h.cert = cfg.SSLCombinedPem
	h.rp = cfg.DefaultRetentionPolicy

	// If a cert is specified, this means the user
	// wants to do HTTPS
	h.schema = "http"
	if h.cert != "" {
		h.schema = "https"
	}

	// For each output specified in the config, we are going to create a backend
	for _, epc := range cfg.Endpoint {
		h.log.Info().Msgf("Processing ENDPOINT %+v", epc)
		ep, err := NewHTTPEndpoint(epc, h.log)
		if err != nil {
			h.log.Err(err)
			return nil, err
		}

		h.Endpoints = append(h.Endpoints, ep)
	}

	for i, b := range h.Endpoints {
		h.log.Info().Msgf("ENDPOINT [%d] | %+v", i, b)
	}

	// If a RateLimit is specified, create a new limiter
	if cfg.RateLimit != 0 {
		if cfg.BurstLimit != 0 {
			h.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.BurstLimit)
		} else {
			h.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)
		}
	}
	return h, nil
}

// Name is the name of the HTTP relay
// a default name might be generated if it is
// not specified in the configuration file
func (h *HTTP) Name() string {
	if h.cfg.Name == "" {
		return fmt.Sprintf("%s://%s", h.schema, h.cfg.BindAddr)
	}
	return h.cfg.Name
}

// Run actually launch the HTTP endpoint
func (h *HTTP) Run() error {
	var cert tls.Certificate
	l, err := net.Listen("tcp", h.cfg.BindAddr)
	if err != nil {
		return err
	}

	// support HTTPS
	if h.cert != "" {
		cert, err = tls.LoadX509KeyPair(h.cert, h.cert)
		if err != nil {
			return err
		}

		l = tls.NewListener(l, &tls.Config{
			Certificates: []tls.Certificate{cert},
		})
	}

	h.l = l

	h.log.Printf("starting %s relay %q on %v", strings.ToUpper(h.schema), h.Name(), h.cfg.BindAddr)

	err = http.Serve(l, h)
	if atomic.LoadInt64(&h.closing) != 0 {
		return nil
	}
	return err
}

// Stop actually stops the HTTP endpoint
func (h *HTTP) Stop() error {
	atomic.StoreInt64(&h.closing, 1)
	return h.l.Close()
}

func (h *HTTP) handlePing(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := GetCtxParam(r, "clusterid")
	//if c, ok := clusters[clusterid.(string)]; ok {
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Ping for cluster %s", clusterid)
		c.HandlePing(w, r, start)
		return
	}
	h.log.Error().Msgf("Handle Ping for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleStatus(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := GetCtxParam(r, "clusterid")
	//clusterid := r.Context().Value("clusterid")
	//if c, ok := clusters[clusterid.(string)]; ok {
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Status for cluster %s", clusterid)
		c.HandleStatus(w, r, start)
		return
	}
	h.log.Error().Msgf("Handle Status for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleHealth(w http.ResponseWriter, r *http.Request, start time.Time) {
	clusterid := GetCtxParam(r, "clusterid")
	//clusterid := r.Context().Value("clusterid")
	//if c, ok := clusters[clusterid.(string)]; ok {
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle Health for cluster %s", clusterid)
		c.HandleHealth(w, r, start)
		return
	}
	h.log.Error().Msgf("Handle Health for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) handleFlush(w http.ResponseWriter, r *http.Request, start time.Time) {
	//clusterid := r.Context().Value("clusterid")
	clusterid := GetCtxParam(r, "clusterid")
	//if c, ok := clusters[clusterid.(string)]; ok {
	if c, ok := clusters[clusterid]; ok {
		h.log.Info().Msgf("Handle flush for cluster %s", clusterid)
		c.HandleFlush(w, r, start)
		return
	}
	h.log.Error().Msgf("Handle Flush for cluster Error cluster %s not exist", clusterid)
}

func (h *HTTP) processEndpoint(w http.ResponseWriter, r *http.Request, start time.Time) {
	// Begin process for
	for i, endpoint := range h.Endpoints {
		h.log.Debug().Msgf("Processing [%d][%s] endpoint %+v", i, endpoint.cfg.Type, endpoint.cfg.URI)
		processed := endpoint.ProcessInput(w, r, start)
		if processed {
			break
		}
	}
}

var ProcessEndpoint relayHandlerFunc = (*HTTP).processEndpoint

// ServeHTTP is the function that handles the different route
// The response is a JSON object describing the state of the operation
func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	R := InitRelayContext(r)

	AppendCxtTracePath(R, "http", h.cfg.Name)

	h.log.Debug().Msgf("IN REQUEST:%+v", R)

	for url, fun := range handlers {
		if strings.HasPrefix(r.URL.Path, url) {
			clusterid := strings.TrimPrefix(R.URL.Path, url+"/")
			SetCtxParam(R, "clusterid", clusterid)

			allMiddlewares(h, fun)(h, w, R, time.Now())
			return
		}

	}
	allMiddlewares(h, ProcessEndpoint)(h, w, R, time.Now())
	//TODO: process allMiddleware before handlers (most probable /write /read than /admin /health)
}
