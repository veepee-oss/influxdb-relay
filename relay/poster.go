package relay

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type poster interface {
	post([]byte, string, string, string) (*responseData, error)
	query(string, string, string) (*http.Response, error)
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
	ret := &responseData{id: s.location}
	req, err := http.NewRequest("POST", s.location+endpoint, bytes.NewReader(buf))
	if err != nil {
		return ret, err
	}

	req.URL.RawQuery = query
	req.Header.Set("Content-Type", "text/plain")
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

func (s *simplePoster) query(params string, authHeader string, endpoint string) (*http.Response, error) {

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
