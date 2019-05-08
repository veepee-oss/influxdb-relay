package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/time/rate"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/toni-moreno/influxdb-srelay/config"
)

type health struct {
	name     string
	err      error
	duration time.Duration
}

type healthReport struct {
	Status  string            `json:"status"`
	Healthy map[string]string `json:"healthy,omitempty"`
	Problem map[string]string `json:"problem,omitempty"`
}

type Cluster struct {
	cfg *config.Influxcluster

	pingResponseCode    int
	pingResponseHeaders map[string]string

	closing int64

	backends []*dbBackend
	log      *zerolog.Logger

	rateLimiter *rate.Limiter

	healthTimeout time.Duration

	queryRouterEndpointAPI []string

	WriteHTTP func(w http.ResponseWriter, r *http.Request, start time.Time) []*responseData
	WriteData func(w http.ResponseWriter, params *InfluxParams, data *bytes.Buffer) []*responseData
	QueryHTTP func(w http.ResponseWriter, r *http.Request, start time.Time)

	bufPool sync.Pool
}

// Buffer Pool

// ErrBufferFull error indicates that retry buffer is full
var ErrBufferFull = errors.New("retry buffer full")

func (c *Cluster) getBuf() *bytes.Buffer {
	if bb, ok := c.bufPool.Get().(*bytes.Buffer); ok {
		return bb
	}
	return new(bytes.Buffer)
}

func (c *Cluster) putBuf(b *bytes.Buffer) {
	b.Reset()
	c.bufPool.Put(b)
}

func NewCluster(cfg *config.Influxcluster) (*Cluster, error) {
	c := new(Cluster)
	c.cfg = cfg

	c.bufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

	//Log output

	c.log = GetConsoleLogFormated(cfg.LogFile, cfg.LogLevel)

	switch c.cfg.Type {
	case "HA":
		c.WriteHTTP = c.handleWriteHA
		c.WriteData = c.handleWriteDataHA
		c.QueryHTTP = c.handleQueryHA
	case "Single", "SINGLE":
		c.WriteHTTP = c.handleWriteSingle
		c.WriteData = c.handleWriteDataSingle
		c.QueryHTTP = c.handleQuerySingle
	default:
		c.WriteHTTP = c.handleWriteSingle
		c.WriteData = c.handleWriteDataSingle
		c.QueryHTTP = c.handleQuerySingle
	}

	//check url is ok //pending a first query
	for _, r := range c.queryRouterEndpointAPI {
		_, err := url.ParseRequestURI(r)
		if err != nil {
			return c, err
		}
	}

	// For each output specified in the config, we are going to create a backend
	for _, beName := range cfg.Members {
		becfg := mainConfig.GetInfluxDBBackend(beName)
		if becfg == nil {
			log.Error().Msgf("Can not find config for cluster member %s", beName)
			return c, errors.New("Can not find config for cluster member " + beName)
		}
		c.log.Debug().Msgf("Config Cluster %s member: %s [%+v]", cfg.Name, beName, becfg)

		backend, err := NewDBBackend(becfg, c.log, c.cfg.Name)
		if err != nil {
			return c, err
		}

		c.backends = append(c.backends, backend)
	}

	c.pingResponseCode = DefaultHTTPPingResponse
	if cfg.DefaultPingResponse != 0 {
		c.pingResponseCode = cfg.DefaultPingResponse
	}

	c.pingResponseHeaders = make(map[string]string)
	c.pingResponseHeaders["X-InfluxDB-Version"] = "smart-relay"
	if c.pingResponseCode != http.StatusNoContent {
		c.pingResponseHeaders["Content-Length"] = "0"
	}

	// If a RateLimit is specified, create a new limiter
	if cfg.RateLimit != 0 {
		if cfg.BurstLimit != 0 {
			c.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.BurstLimit)
		} else {
			c.rateLimiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)
		}
	}

	c.healthTimeout = time.Duration(cfg.HealthTimeout) * time.Millisecond
	return c, nil
}

func (c *Cluster) HandlePing(w http.ResponseWriter, r *http.Request, _ time.Time) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		for key, value := range c.pingResponseHeaders {
			w.Header().Add(key, value)
		}
		w.WriteHeader(c.pingResponseCode)
	} else {
		jsonResponse(w, r, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
}

func (c *Cluster) HandleHealth(w http.ResponseWriter, r *http.Request, _ time.Time) {
	var responses = make(chan health, len(c.backends))
	var wg sync.WaitGroup
	var validEndpoints = 0
	wg.Add(len(c.backends))

	for _, b := range c.backends {
		b := b

		validEndpoints++

		go func() {
			defer wg.Done()
			var healthCheck = health{name: b.cfg.Name, err: nil}

			client := http.Client{
				Timeout: c.healthTimeout,
			}
			start := time.Now()
			res, err := client.Get(b.cfg.Location + "ping")

			if err != nil {

				c.log.Err(err)
				healthCheck.err = err
				responses <- healthCheck
				return
			}
			if res.StatusCode/100 != 2 {
				healthCheck.err = errors.New("Unexpected error code " + string(res.StatusCode))
			}
			healthCheck.duration = time.Since(start)
			responses <- healthCheck
			return
		}()
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	nbDown := 0
	report := healthReport{}
	for r := range responses {
		if r.err == nil {
			if report.Healthy == nil {
				report.Healthy = make(map[string]string)
			}
			report.Healthy[r.name] = "OK. Time taken " + r.duration.String()

		} else {
			if report.Problem == nil {
				report.Problem = make(map[string]string)
			}
			report.Problem[r.name] = "KO. " + r.err.Error()
			nbDown++
		}
	}
	switch {
	case nbDown == validEndpoints:
		report.Status = "critical"
	case nbDown >= 1:
		report.Status = "problem"
	case nbDown == 0:
		report.Status = "healthy"
	}
	response := response{code: 200, body: report}
	jsonResponse(w, r, response)
	return
}

func (c *Cluster) HandleStatus(w http.ResponseWriter, r *http.Request, _ time.Time) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		// old but gold
		st := make(map[string]map[string]string)

		for _, b := range c.backends {
			st[b.cfg.Name] = b.poster.getStats()
		}

		j, _ := json.Marshal(st)

		jsonResponse(w, r, response{http.StatusOK, fmt.Sprintf("\"status\": %s", string(j))})
	} else {
		jsonResponse(w, r, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
		return
	}
}

func (c *Cluster) HandleFlush(w http.ResponseWriter, r *http.Request, start time.Time) {

	c.log.Info().Msg("Flushing buffers...")

	for _, b := range c.backends {
		r := b.getRetryBuffer()

		if r != nil {
			c.log.Info().Msg("Flushing " + b.cfg.Name)
			c.log.Info().Msg("NOT flushing " + b.cfg.Name + " (is empty)")
			r.empty()
		}
	}

	jsonResponse(w, r, response{http.StatusOK, http.StatusText(http.StatusOK)})
}

func (c *Cluster) handleWriteBase(w http.ResponseWriter, r *http.Request) bool {
	//CHECK METHOD

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
		} else {
			jsonResponse(w, r, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
			return false
		}
	}

	//CHECK RATE

	if c.rateLimiter != nil && !c.rateLimiter.Allow() {
		c.log.Debug().Msgf("Rate Limited => Too Many Request (Limit %+v)(Burst %d) ", c.rateLimiter.Limit(), c.rateLimiter.Burst)
		jsonResponse(w, r, response{http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests)})
		return false
	}
	return true
}

func (c *Cluster) handleWriteSingle(w http.ResponseWriter, r *http.Request, start time.Time) []*responseData {

	// check if can continue

	cont := c.handleWriteBase(w, r)
	if !cont {
		return nil
	}

	queryParams := r.URL.Query()
	bodyBuf := c.getBuf()
	_, _ = bodyBuf.ReadFrom(r.Body)

	// done with the input points
	// normalize query string
	query := queryParams.Encode()

	outBytes := bodyBuf.Bytes()
	SetCtxRequestSize(r, bodyBuf.Len(), -1)

	c.log.Info().Msgf("Content Length BODYBUF: %d", len(bodyBuf.String()))

	// check for authorization performed via the header
	authHeader := r.Header.Get("Authorization")

	b := c.backends[0]
	resp, err := b.post(outBytes, query, authHeader, "write")
	if err != nil {
		c.log.Info().Msgf("Problem posting to cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, err)
	} else {
		if resp.StatusCode/100 == 5 {
			c.log.Info().Msgf("5xx response for cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, resp.StatusCode)
		}
	}

	c.putBuf(bodyBuf)

	return []*responseData{resp}
}

func (c *Cluster) handleWriteHA(w http.ResponseWriter, r *http.Request, start time.Time) []*responseData {

	AppendCxtTracePath(r, "handleWriteHA", c.cfg.Name)
	// check if can continue

	cont := c.handleWriteBase(w, r)
	if !cont {
		return nil
	}

	//Query

	queryParams := r.URL.Query()
	bodyBuf := c.getBuf()
	_, _ = bodyBuf.ReadFrom(r.Body)

	query := queryParams.Encode()

	outBytes := bodyBuf.Bytes()

	c.log.Info().Msgf("Content Length BODYBUF: %d", len(bodyBuf.String()))
	// check for authorization performed via the header
	authHeader := r.Header.Get("Authorization")

	var wg sync.WaitGroup
	wg.Add(len(c.backends))
	SetCtxRequestSize(r, bodyBuf.Len(), -1)

	var responses = make(chan *responseData, len(c.backends))

	for _, b := range c.backends {
		b := b

		go func() {
			defer wg.Done()
			resp, err := b.post(outBytes, query, authHeader, "write")
			if err != nil {
				c.log.Info().Msgf("Problem posting to cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, err)
				responses <- &responseData{}
			} else {
				if resp.StatusCode/100 == 5 {
					c.log.Info().Msgf("5xx response for cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, resp.StatusCode)
				}
				responses <- resp
			}
		}()
	}

	go func() {
		wg.Wait()
		close(responses)
		c.putBuf(bodyBuf)
	}()

	return ChanToSlice(responses).([]*responseData)
}

func (c *Cluster) handleWriteDataSingle(w http.ResponseWriter, params *InfluxParams, data *bytes.Buffer) []*responseData {

	//AppendCxtTracePath(r, "handleWriteDataSingle", c.cfg.Name)

	b := c.backends[0]
	resp, err := b.post(data.Bytes(), params.QueryEncode(), params.Header["authorization"], "write")
	if err != nil {
		c.log.Info().Msgf("Problem posting to cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, err)
	} else {
		if resp.StatusCode/100 == 5 {
			c.log.Info().Msgf("5xx response for cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, resp.StatusCode)
		}
	}

	return []*responseData{resp}
}

func (c *Cluster) handleWriteDataHA(w http.ResponseWriter, params *InfluxParams, data *bytes.Buffer) []*responseData {

	//AppendCxtTracePath(r, "handleWriteDataHA", c.cfg.Name)

	var wg sync.WaitGroup
	wg.Add(len(c.backends))

	var responses = make(chan *responseData, len(c.backends))

	encode := params.QueryEncode()
	databytes := data.Bytes()
	auth := params.Header["authorization"]

	for _, b := range c.backends {
		b := b

		go func() {
			defer wg.Done()
			c.log.Debug().Msgf("Writing data on %s Data length %d", b.cfg.Name, len(databytes))
			resp, err := b.post(databytes, encode, auth, "write")
			if err != nil {
				c.log.Info().Msgf("Problem posting to cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, err)
				responses <- &responseData{}
			} else {
				if resp.StatusCode/100 == 5 {
					c.log.Info().Msgf("5xx response for cluster %q backend %q: %v", c.cfg.Name, b.cfg.Name, resp.StatusCode)
				}
				c.log.Debug().Msgf("RESPONSE %+v: Body : %s", resp, string(resp.Body))
				responses <- resp
			}
		}()
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	return ChanToSlice(responses).([]*responseData)
}

func (c *Cluster) getValidQueryBackend() *dbBackend {
	nEndp := len(c.queryRouterEndpointAPI)
	if nEndp == 0 {
		return c.backends[0]
	}
	var responses = make(chan []string, nEndp)
	var wg sync.WaitGroup
	var validEndpoints = 0
	wg.Add(nEndp)

	for _, r := range c.queryRouterEndpointAPI {

		validEndpoints++

		go func(r string) {
			defer wg.Done()

			client := http.Client{
				Timeout: c.healthTimeout,
			}
			start := time.Now()
			resp, err := client.Get(r)

			if err != nil {
				c.log.Err(err)
				return
			}
			defer resp.Body.Close()
			var array []string
			if resp.StatusCode == http.StatusOK {
				bodyBytes, _ := ioutil.ReadAll(resp.Body)

				err := json.Unmarshal(bodyBytes, &array)
				if err != nil {
					c.log.Printf("Error  %#+v\n", bodyBytes, err)
				}
				responses <- array
			}
			duration := time.Since(start)
			c.log.Printf("Response %#+v | Duration  %s\n", array, duration.String())
			return
		}(r)
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	var allEndpoints []string

	for r := range responses {
		allEndpoints = append(allEndpoints, r...)
	}

	var rehttp *dbBackend

	if len(allEndpoints) > 0 {
		for _, b := range c.backends {
			if b.cfg.Name == allEndpoints[0] {
				rehttp = b
			}
		}
	}
	if rehttp == nil {
		rehttp = c.backends[0]
	}
	return rehttp
}

func (c *Cluster) handleQueryHA(w http.ResponseWriter, r *http.Request, start time.Time) {
	AppendCxtTracePath(r, "handleQueryHA", c.cfg.Name)
	if r.Method != http.MethodPost && r.Method != http.MethodGet && r.Method != http.MethodHead {
		//w.Header().Set("Allow", http.MethodPost)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
		} else {
			jsonResponse(w, r, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
			return
		}
	}
	b := c.getValidQueryBackend()
	c.handleQuery(w, r, start, b)
}

func (c *Cluster) handleQuerySingle(w http.ResponseWriter, r *http.Request, start time.Time) {
	AppendCxtTracePath(r, "handleQuerySingle", c.cfg.Name)
	if r.Method != http.MethodPost && r.Method != http.MethodGet && r.Method != http.MethodHead {
		//w.Header().Set("Allow", http.MethodPost)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
		} else {
			jsonResponse(w, r, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
			return
		}
	}
	b := c.backends[0]
	c.handleQuery(w, r, start, b)
}

func (c *Cluster) handleQuery(w http.ResponseWriter, r *http.Request, start time.Time, b *dbBackend) {

	queryParams := r.URL.Query()
	c.log.Debug().Msgf("QUERY PARAMS: %+v	", queryParams)

	paramString := queryParams.Encode()
	authHeader := r.Header.Get("Authorization")

	resp, err := b.query(paramString, authHeader, "query")
	if err != nil {
		c.log.Error().Msgf("Problem posting to cluster %s backend %s: %s", c.cfg.Name, b.cfg.Name, err)
	}

	for name, values := range resp.Header {
		w.Header()[name] = values
	}

	w.WriteHeader(resp.StatusCode)

	length, _ := io.Copy(w, resp.Body)
	SetCtxRequestSentParams(r, int(resp.StatusCode), int(length))

	//c.log.Info().Msgf("IN QUERY HTTP CODE [%d] | Auth(%s) | Query [%s]  Response Time (%s)\n", resp.StatusCode, queryParams.Encode(), authHeader, time.Since(start))

}
