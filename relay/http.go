package relay

import (
	"crypto/tls"
	"fmt"
	"golang.org/x/time/rate"

	"net"
	"net/http"

	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayctx"
	"github.com/toni-moreno/influxdb-srelay/utils"
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

type relayHandlerFunc func(h *HTTP, w http.ResponseWriter, r *http.Request)
type relayMiddleware func(h *HTTP, handlerFunc relayHandlerFunc) relayHandlerFunc

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

	h.log = utils.GetConsoleLogFormated(cfg.LogFile, cfg.LogLevel)
	//AccessLog Output

	h.acclog = utils.GetConsoleLogFormated(cfg.AccessLog, "debug")

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
		h.log.Info().Msg("Relay Run, closing....")
		return nil
	}
	return err
}

// Stop actually stops the HTTP endpoint
func (h *HTTP) Stop() error {
	atomic.StoreInt64(&h.closing, 1)
	return h.l.Close()
}

func (h *HTTP) processEndpoint(w http.ResponseWriter, r *http.Request) {
	// Begin process for
	for i, endpoint := range h.Endpoints {
		h.log.Debug().Msgf("Processing [%d][%s] endpoint %+v", i, endpoint.cfg.Type, endpoint.cfg.URI)
		processed := endpoint.ProcessInput(w, r)
		if processed {
			break
		}
	}
}

var ProcessEndpoint relayHandlerFunc = (*HTTP).processEndpoint

// ServeHTTP is the function that handles the different route
// The response is a JSON object describing the state of the operation
func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	R := relayctx.InitRelayContext(r)

	relayctx.AppendCxtTracePath(R, "http", h.cfg.Name)

	h.log.Debug().Msgf("IN REQUEST:%+v", R)
	//first we will try to process user EndPoints
	allMiddlewares(h, ProcessEndpoint)(h, w, R)
	//if not served we will process the amdin EndPoints
	served := relayctx.GetServed(R)
	if !served {
		h.log.Debug().Msg("Query not served by user endpoints, reviewing relay handlers ")
		for url, fun := range handlers {
			if strings.HasPrefix(r.URL.Path, url) {
				var clusterid string
				if url == R.URL.Path {
					clusterid = ""
				} else {
					clusterid = strings.TrimPrefix(R.URL.Path, url+"/")
				}

				h.log.Debug().Msgf("handle r.URL.Path for CLUSTERID %s", clusterid)
				relayctx.SetCtxParam(R, "clusterid", clusterid)

				allMiddlewares(h, fun)(h, w, R)
				return
			}

		}
	}

}
