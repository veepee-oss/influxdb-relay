package backend

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type poster interface {
	Post([]byte, string, string, string, string) (*ResponseData, error)
	Query(string, string, string) (*http.Response, error)
	GetStats() map[string]string
}

type simplePoster struct {
	serverid  string
	clusterid string
	client    *http.Client
	location  string
}

func newSimplePoster(serverid string, location string, clusterid string, timeout time.Duration, skipTLSVerification bool) *simplePoster {
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
		location:  location,
		serverid:  serverid,
		clusterid: clusterid,
	}
}

func (s *simplePoster) GetStats() map[string]string {
	v := make(map[string]string)
	v["location"] = s.location
	return v
}

func (s *simplePoster) Post(buf []byte, query string, auth string, endpoint string, ctype string) (*ResponseData, error) {
	ret := &ResponseData{Serverid: s.serverid, Clusterid: s.clusterid, Location: s.location}
	req, err := http.NewRequest("POST", s.location+endpoint, bytes.NewReader(buf))
	if err != nil {
		return ret, err
	}

	req.URL.RawQuery = query //<-Review
	if len(ctype) > 0 {
		req.Header.Set("Content-Type", ctype)
	} else {
		req.Header.Set("Content-Type", "text/plain")
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(buf)))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("User-Agent", "influxdb-smart-relay")

	resp, err := s.client.Do(req)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ret, err
	}
	ret.ContentEncoding = resp.Header.Get("Content-Encoding")
	ret.ContentType = resp.Header.Get("Content-Type")
	ret.StatusCode = resp.StatusCode
	ret.Body = data
	return ret, nil
}

func (s *simplePoster) Query(params string, authHeader string, endpoint string) (*http.Response, error) {

	req, err := http.NewRequest("GET", s.location+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = params

	req.Header.Set("User-Agent", "influxdb-smart-relay")

	req.Header.Add("Accept", "application/json")
	if len(authHeader) > 0 {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
