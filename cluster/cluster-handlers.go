package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
	"sync"
	"time"

	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/relayctx"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

func (c *Cluster) HandlePing(w http.ResponseWriter, r *http.Request, _ time.Time) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		//TODO do a real ping over all cluster members
		utils.AddInfluxPingHeaders(w, "Influx-Smart-Relay")
		w.WriteHeader(c.pingResponseCode)
	} else {
		relayctx.JsonResponse(w, r, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
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
			var healthCheck = health{name: b.Name(), err: nil}

			client := http.Client{
				Timeout: c.healthTimeout,
			}
			start := time.Now()
			res, err := client.Get(b.URL("ping"))

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

	relayctx.JsonResponse(w, r, 200, report)
	return
}

func (c *Cluster) HandleStatus(w http.ResponseWriter, r *http.Request, _ time.Time) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		// old but gold
		st := make(map[string]map[string]string)

		for _, b := range c.backends {
			st[b.Name()] = b.GetStats()
		}

		j, _ := json.Marshal(st)

		relayctx.JsonResponse(w, r, http.StatusOK, fmt.Sprintf("\"status\": %s", string(j)))
	} else {
		relayctx.JsonResponse(w, r, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}
}

func (c *Cluster) HandleFlush(w http.ResponseWriter, r *http.Request, start time.Time) {

	c.log.Info().Msg("Flushing buffers...")

	for _, b := range c.backends {
		r := b.GetRetryBuffer()

		if r != nil {
			c.log.Info().Msg("Flushing " + b.Name())
			c.log.Info().Msg("NOT flushing " + b.Name() + " (is empty)")
			r.Empty()
		}
	}

	relayctx.JsonResponse(w, r, http.StatusOK, http.StatusText(http.StatusOK))
}

type AdminResult struct {
	Msg       string
	Responses []*backend.ResponseData
}

func (c *Cluster) HandleAdmin(w http.ResponseWriter, r *http.Request, _ time.Time) {

	if r.Method != http.MethodPost {
		// Bad method
		w.Header().Set("Allow", http.MethodPost)
		relayctx.JsonResponse(w, r, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	bodyBuf := bytes.Buffer{}
	_, _ = bodyBuf.ReadFrom(r.Body)
	queryParams := r.URL.Query()
	query := queryParams.Encode()
	authHeader := r.Header.Get("Authorization")

	// Responses
	var responses = make(chan *backend.ResponseData, len(c.backends))

	// Associated waitgroup
	var wg sync.WaitGroup
	wg.Add(len(c.backends))

	c.log.Debug().Msgf("Query BODY %s  QUERY %s AUTH", bodyBuf.String(), query, authHeader)
	// Iterate over all backends
	for _, b := range c.backends {
		b := b

		go func() {
			defer wg.Done()

			resp, err := b.Post(bodyBuf.Bytes(), query, authHeader, "query", "application/x-www-form-urlencoded")
			if err != nil {
				c.log.Info().Msgf("Problem posting to cluster %q backend %q: %v", c.cfg.Name, b.Name(), err)
				responses <- &backend.ResponseData{}
			} else {
				if resp.StatusCode/100 == 5 {
					c.log.Info().Msgf("5xx response for cluster %q backend %q: %v", c.cfg.Name, b.Name(), resp.StatusCode)
				}
				responses <- resp
				c.log.Debug().Msgf("Response From %s : StatusCode %d : %s", b.Name(), resp.StatusCode, string(resp.Body))
			}
		}()

	}

	// Wait for requests
	go func() {
		wg.Wait()
		close(responses)
	}()
	adminresp := &AdminResult{}
	var errResponse *backend.ResponseData
	for resp := range responses {
		adminresp.Responses = append(adminresp.Responses, resp)
		c.log.Debug().Msgf("Responses %+v", resp)
		switch resp.StatusCode / 100 {

		case 4, 5:
			// User error
			errResponse = resp
			break

		default:
			// Hold on to one of the responses to return back to the client
			errResponse = nil
		}
	}

	// No error !! The
	if errResponse == nil {
		adminresp.Msg = fmt.Sprintf("Cluster %s : Admin Action  %s: OK", c.cfg.Name, bodyBuf.String())
		// Failed to make any valid request...
		relayctx.JsonResponse(w, r, 200, adminresp)
		return
	}
	adminresp.Msg = fmt.Sprintf("Cluster %s : Admin Action  %s: ERROR on ", c.cfg.Name, bodyBuf.String(), errResponse.Serverid)
	relayctx.JsonResponse(w, r, http.StatusBadRequest, adminresp)
}
