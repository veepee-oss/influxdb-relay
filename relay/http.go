package relay

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/influxdb/models"
	"github.com/vente-privee/influxdb-relay/config"
)

// HTTP is a relay for HTTP influxdb writes
type HTTP struct {
	addr   string
	name   string
	schema string

	cert string
	rp   string

	pingResponseCode    int
	pingResponseHeaders map[string]string

	closing int64
	l       net.Listener

	backends []*httpBackend

	start  time.Time
	log    bool
	logger *log.Logger

	rateLimiter *rate.Limiter

	healthTimeout time.Duration

	queryRouterEndpointApi []string
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
		"/write":             (*HTTP).handleStandard,
		"/api/v1/prom/write": (*HTTP).handleProm,
		"/ping":              (*HTTP).handlePing,
		"/status":            (*HTTP).handleStatus,
		"/admin":             (*HTTP).handleAdmin,
		"/admin/flush":       (*HTTP).handleFlush,
		"/health":            (*HTTP).handleHealth,
		"/query":             (*HTTP).handleQuery,
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
func NewHTTP(cfg config.HTTPConfig, verbose bool, fs config.Filters) (Relay, error) {
	h := new(HTTP)

	h.addr = cfg.Addr
	h.name = cfg.Name
	h.log = verbose
	h.logger = log.New(os.Stdout, "relay: ", 0)

	h.queryRouterEndpointApi = cfg.QueryRouterEndpointApi

	//check url is ok //pending a first query
	for _, r := range h.queryRouterEndpointApi {
		_, err := url.ParseRequestURI(r)
		if err != nil {
			return nil, err
		}
	}

	h.pingResponseCode = DefaultHTTPPingResponse
	if cfg.DefaultPingResponse != 0 {
		h.pingResponseCode = cfg.DefaultPingResponse
	}

	h.pingResponseHeaders = make(map[string]string)
	h.pingResponseHeaders["X-InfluxDB-Version"] = "relay"
	if h.pingResponseCode != http.StatusNoContent {
		h.pingResponseHeaders["Content-Length"] = "0"
	}

	h.cert = cfg.SSLCombinedPem
	h.rp = cfg.DefaultRetentionPolicy

	// If a cert is specified, this means the user
	// wants to do HTTPS
	h.schema = "http"
	if h.cert != "" {
		h.schema = "https"
	}

	// For each output specified in the config, we are going to create a backend
	for i := range cfg.Outputs {
		backend, err := newHTTPBackend(&cfg.Outputs[i], fs)
		if err != nil {
			return nil, err
		}

		h.backends = append(h.backends, backend)
	}

	// If a RateLimit is specified, create a new limiter
	if cfg.RateLimit != 0 {
		if cfg.BurstLimit != 0 {
			h.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.BurstLimit)
		} else {
			h.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)
		}
	}

	h.healthTimeout = time.Duration(cfg.HealthTimeout) * time.Millisecond

	return h, nil
}

// Name is the name of the HTTP relay
// a default name might be generated if it is
// not specified in the configuration file
func (h *HTTP) Name() string {
	if h.name == "" {
		return fmt.Sprintf("%s://%s", h.schema, h.addr)
	}

	return h.name
}

// Run actually launch the HTTP endpoint
func (h *HTTP) Run() error {
	var cert tls.Certificate
	l, err := net.Listen("tcp", h.addr)
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

	if h.log {
		h.logger.Printf("starting %s relay %q on %v", strings.ToUpper(h.schema), h.Name(), h.addr)
	}

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

// ServeHTTP is the function that handles the different route
// The response is a JSON object describing the state of the operation
func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// h.start = time.Now()
	if fun, ok := handlers[r.URL.Path]; ok {
		allMiddlewares(h, fun)(h, w, r, time.Now())
	} else {
		jsonResponse(w, response{http.StatusNotFound, http.StatusText(http.StatusNotFound)})
		return
	}
}

type responseData struct {
	ContentType     string
	ContentEncoding string
	StatusCode      int
	Body            []byte
}

func (rd *responseData) Write(w http.ResponseWriter) {
	if rd.ContentType != "" {
		w.Header().Set("Content-Type", rd.ContentType)
	}

	if rd.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", rd.ContentEncoding)
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(rd.Body)))
	w.WriteHeader(rd.StatusCode)
	w.Write(rd.Body)
}

func jsonResponse(w http.ResponseWriter, r response) {
	data, err := json.Marshal(r.body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(r.code)

	_, _ = w.Write(data)
}

type ifxQuery struct {
	db         string
	epoch      string
	q          string
	u          string
	p          string
	pretty     string
	chunked    string
	chunksize  string
	authHeader string
}

type poster interface {
	post([]byte, string, string, string) (*responseData, error)
	query(*ifxQuery, string) (*http.Response, error)
	getStats() map[string]string
}

type simplePoster struct {
	client   *http.Client
	location string
}

func newSimplePoster(location string, timeout time.Duration, skipTLSVerification bool) *simplePoster {
	// Configure custom transport for http.Client
	// Used for support skip-tls-verification option
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerification,
		},
	}

	return &simplePoster{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		location: location,
	}
}

func (s *simplePoster) getStats() map[string]string {
	v := make(map[string]string)
	v["location"] = s.location
	return v
}

func (s *simplePoster) post(buf []byte, query string, auth string, endpoint string) (*responseData, error) {
	req, err := http.NewRequest("POST", s.location+endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = query
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Length", strconv.Itoa(len(buf)))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &responseData{
		ContentType:     resp.Header.Get("Content-Type"),
		ContentEncoding: resp.Header.Get("Content-Encoding"),
		StatusCode:      resp.StatusCode,
		Body:            data,
	}, nil
}

//acording to
//  https://docs.influxdata.com/influxdb/v1.7/guides/querying_data/#querying-data-with-the-http-api

func (s *simplePoster) query(inq *ifxQuery, endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", s.location+endpoint, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	if len(inq.db) > 0 {
		q.Add("db", inq.db)
	}
	if len(inq.epoch) > 0 {
		q.Add("epoch", inq.epoch)
	}
	if len(inq.q) > 0 {
		q.Add("q", inq.q)
	}
	if len(inq.u) > 0 {
		q.Add("u", inq.u)
	}
	if len(inq.p) > 0 {
		q.Add("p", inq.p)
	}
	if len(inq.pretty) > 0 {
		q.Add("pretty", inq.pretty)
	}
	if len(inq.chunked) > 0 {
		q.Add("chunked", inq.chunked)
	}

	req.URL.RawQuery = q.Encode()

	req.Header.Add("Accept", "application/json")
	if len(inq.authHeader) > 0 {
		req.Header.Set("Authorization", inq.authHeader)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type httpBackend struct {
	poster
	name      string
	inputType config.Input
	admin     string
	endpoints config.HTTPEndpointConfig
	location  string

	tagRegexps         []*regexp.Regexp
	measurementRegexps []*regexp.Regexp
}

// validateRegexps checks if a request on this backend matches
// all the tag regular expressions for this backend
func (b *httpBackend) validateRegexps(ps models.Points) error {
	// For each point
	for _, p := range ps {
		// Check if the measurement of each point
		// matches ALL measurement regular expressions
		m := p.Name()
		for _, r := range b.measurementRegexps {
			if !r.Match(m) {
				return errors.New("bad measurement")
			}
		}

		// For each tag of each point
		for _, t := range p.Tags() {
			// Check if each tag of each point
			// matches ALL tags regular expressions
			for _, r := range b.tagRegexps {
				if !r.Match(t.Key) {
					return errors.New("bad tag")
				}
			}
		}
	}

	return nil
}

func (b *httpBackend) getRetryBuffer() *retryBuffer {
	if p, ok := b.poster.(*retryBuffer); ok {
		return p
	}
	return nil
}

func newHTTPBackend(cfg *config.HTTPOutputConfig, fs config.Filters) (*httpBackend, error) {
	// Get default name
	if cfg.Name == "" {
		cfg.Name = cfg.Location
	}

	// Set a timeout
	timeout := DefaultHTTPTimeout
	if cfg.Timeout != "" {
		t, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("error parsing HTTP timeout '%v'", err)
		}
		timeout = t
	}

	// Get underlying Poster instance
	var p poster = newSimplePoster(cfg.Location, timeout, cfg.SkipTLSVerification)

	// If configured, create a retryBuffer per backend.
	// This way we serialize retries against each backend.
	if cfg.BufferSizeMB > 0 {
		max := DefaultMaxDelayInterval
		if cfg.MaxDelayInterval != "" {
			m, err := time.ParseDuration(cfg.MaxDelayInterval)
			if err != nil {
				return nil, fmt.Errorf("error parsing max retry time %v", err)
			}
			max = m
		}

		batch := DefaultBatchSizeKB * KB
		if cfg.MaxBatchKB > 0 {
			batch = cfg.MaxBatchKB * KB
		}

		p = newRetryBuffer(cfg.BufferSizeMB*MB, batch, max, p)
	}

	var tagRegexps []*regexp.Regexp
	var measurementRegexps []*regexp.Regexp

	// Get regexps related to this HTTP backend
	for _, f := range fs {
		for _, e := range f.Outputs {
			if e == cfg.Name {
				if f.TagRegexp != nil {
					tagRegexps = append(tagRegexps, f.TagRegexp)
				}

				if f.MeasurementRegexp != nil {
					measurementRegexps = append(measurementRegexps, f.MeasurementRegexp)
				}
			}
		}
	}

	return &httpBackend{
		poster:             p,
		name:               cfg.Name,
		tagRegexps:         tagRegexps,
		measurementRegexps: measurementRegexps,
		endpoints:          cfg.Endpoints,
		location:           cfg.Location,
	}, nil
}

// ErrBufferFull error indicates that retry buffer is full
var ErrBufferFull = errors.New("retry buffer full")

var bufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

func getBuf() *bytes.Buffer {
	if bb, ok := bufPool.Get().(*bytes.Buffer); ok {
		return bb
	}
	return new(bytes.Buffer)
}

func putBuf(b *bytes.Buffer) {
	b.Reset()
	bufPool.Put(b)
}
