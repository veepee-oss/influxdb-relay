package relay

import (
	"net/http"
)

type InfluxParams struct {
	Header map[string]string
	Query  map[string]string
}

/*
##### HTTP headers
# IP origen <-
# Autorization
###### Influx #######
# db
# u/p
#####################
#  INFLUX QUERY
#####################
# epoch
# chunked/chunksize
# q
# pretty
#####################
# INFLUX WRITE
#####################
# precision
# consistency
# rp
#####################
# PROM WRITE
#####################
*/

//
func SplitParamsIQL(r *http.Request) *InfluxParams {

	queryParams := r.URL.Query()

	return &InfluxParams{
		Header: map[string]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": r.RemoteAddr,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
		},
		Query: map[string]string{
			"db":        queryParams.Get("db"),
			"q":         queryParams.Get("q"),
			"epoch":     queryParams.Get("epoch"),
			"chunked":   queryParams.Get("chunked"),
			"chunksize": queryParams.Get("chunksize"),
			"pretty":    queryParams.Get("pretty"),
			"u":         queryParams.Get("u"),
			"p":         queryParams.Get("p"),
		},
	}
}

//InfluxLineProtocol

func SplitParamsILP(r *http.Request) *InfluxParams {

	queryParams := r.URL.Query()

	return &InfluxParams{
		Header: map[string]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": r.RemoteAddr,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
		},
		Query: map[string]string{
			"db":          queryParams.Get("db"),
			"precision":   queryParams.Get("precision"),
			"consistency": queryParams.Get("consistency"),
			"rp":          queryParams.Get("rp"),
			"u":           queryParams.Get("u"),
			"p":           queryParams.Get("p"),
		},
	}
}

// Prometheus Write parameters

func SplitParamsPRW(r *http.Request) *InfluxParams {

	queryParams := r.URL.Query()

	return &InfluxParams{
		Header: map[string]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": r.RemoteAddr,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
		},
		Query: map[string]string{
			"db":          queryParams.Get("db"),
			"precision":   queryParams.Get("precision"),
			"consistency": queryParams.Get("consistency"),
			"rp":          queryParams.Get("rp"),
			"u":           queryParams.Get("u"),
			"p":           queryParams.Get("p"),
		},
	}
}

func ReMapRequest(r *http.Request, params *InfluxParams, path string) *http.Request {

	//remap only Query parameters
	values := r.URL.Query()
	for qpName, qpValue := range params.Query {
		if len(qpValue) > 0 {
			//check if already exist
			existing := values.Get(qpName)
			if len(existing) > 0 {
				// if exist delete
				values.Del(qpName)
			}
			values.Add(qpName, qpValue)
		}
	}
	r.URL.RawQuery = values.Encode()
	if len(path) > 0 {
		r.Header.Add("X-SmartRelay-Path", path)
	}
	return r
}
