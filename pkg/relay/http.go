package relay

import (
	//"crypto/tls"
	"fmt"

	"golang.org/x/time/rate"

	//"net"
	"net/http"

	"strings"
	"time"

	"context"

	"github.com/rs/zerolog"
	"github.com/toni-moreno/influxdb-srelay/pkg/config"
	"github.com/toni-moreno/influxdb-srelay/pkg/relayctx"
	"github.com/toni-moreno/influxdb-srelay/pkg/utils"
)

// HTTP is a relay for HTTP influxdb writes
type HTTP struct {
	cfg    *config.HTTPConfig
	schema string

	rp string

	pingResponseCode    int
	pingResponseHeaders map[string]string

	closing chan bool

	s *http.Server

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
func NewHTTP(cfg *config.HTTPConfig) (*HTTP, error) {
	h := &HTTP{}
	h.cfg = cfg
	h.closing = make(chan bool, 1)
	//Log output

	h.log = utils.GetConsoleLogFormated(cfg.LogFile, cfg.LogLevel)
	//AccessLog Output

	h.acclog = utils.GetConsoleLogFormated(cfg.AccessLog, "debug")

	h.rp = cfg.DefaultRetentionPolicy

	// If a cert is specified, this means the user
	// wants to do HTTPS
	h.schema = "http"
	if h.cfg.TLSCert != "" {
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

func (h *HTTP) Release() {
	// For each output specified in the config, we are going to create a backend
	for _, ep := range h.Endpoints {
		h.log.Info().Msgf("Releasing ENDPOINT %+v", ep.cfg.URI)
		ep.Release()
	}
	h.Endpoints = nil
	h.log = nil
	h.acclog = nil
	h.rateLimiter = nil
	h.cfg = nil
	h.s = nil
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

	h.s = &http.Server{Addr: h.cfg.BindAddr, Handler: h}
	var err error
	if h.cfg.TLSCert != "" {
		err = h.s.ListenAndServeTLS(h.cfg.TLSCert, h.cfg.TLSKey)
	} else {
		err = h.s.ListenAndServe()
	}
	if err == nil || err == http.ErrServerClosed {
		start := time.Now()
		h.log.Info().Msg("Server Listen Stopping...")
		<-h.closing
		h.log.Info().Msgf("Server Listen Stopped.. in %s", time.Since(start).String())
	}
	return err
}

// Stop actually stops the HTTP endpoint
func (h *HTTP) Stop() error {
	h.log.Info().Msgf("Shutting down the server...%s", h.Name())

	ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)

	err := h.s.Shutdown(ctx)
	close(h.closing)
	h.log.Info().Msg("Server gracefully stopped")
	return err
}

func (h *HTTP) processUnknown(w http.ResponseWriter, r *http.Request) {
	// Begin process for
	relayctx.SetServedOK(r)
	relayctx.SetBackendTime(r)
	relayctx.VoidResponse(w, r, 400)
}

var ProcessUnknown relayHandlerFunc = (*HTTP).processUnknown

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
				relayctx.SetServedOK(R)
				relayctx.SetCtxParam(R, "clusterid", clusterid)
				allMiddlewares(h, fun)(h, w, R)
				return
			}

		}
		served2 := relayctx.GetServed(R)
		if !served2 {
			//NOT SERVED YET
			allMiddlewares(h, ProcessUnknown)(h, w, R)
		}

	}

}
