package backend

import (
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/influxdata/influxdb/models"

	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

type InfluxParams struct {
	Header map[config.RuleKey]string
	Query  map[config.RuleKey]string
	Points models.Points
}

func (ip *InfluxParams) Clone() *InfluxParams {
	return &InfluxParams{
		Header: map[config.RuleKey]string{
			"authorization":  ip.Header["authorization"],
			"remote-address": ip.Header["remote-address"],
			"referer":        ip.Header["referer"],
			"user-agent":     ip.Header["user-agent"],
			"username":       ip.Header["username"],
		},
		Query: map[config.RuleKey]string{
			"db":        ip.Query["db"],
			"q":         ip.Query["q"],
			"epoch":     ip.Query["epoch"],
			"chunked":   ip.Query["chunked"],
			"chunksize": ip.Query["chunksize"],
			"pretty":    ip.Query["pretty"],
			"u":         ip.Query["u"],
			"p":         ip.Query["p"],
		},
	}
}

func (ip *InfluxParams) QueryEncode() string {
	if ip == nil {
		return ""
	}
	v := ip.Query
	var buf strings.Builder
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, string(k))
	}

	sort.Strings(keys)

	for _, k := range keys {
		vs := v[config.RuleKey(k)]
		keyEscaped := url.QueryEscape(string(k))
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(keyEscaped)
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(vs))
	}
	return buf.String()
}

func (ip *InfluxParams) SetDB(db string) {
	ip.Query["db"] = db
}

func SplitParamsIQL(r *http.Request) *InfluxParams {

	queryParams := r.URL.Query()
	ipsource, ipfwd := utils.GetSourceFromRequest(r)

	return &InfluxParams{
		Header: map[config.RuleKey]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": ipsource,
			"fwd-address":    ipfwd,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
			"username":       utils.GetUserFromRequest(r),
		},
		Query: map[config.RuleKey]string{
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
	ipsource, ipfwd := utils.GetSourceFromRequest(r)

	return &InfluxParams{
		Header: map[config.RuleKey]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": ipsource,
			"fwd-address":    ipfwd,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
			"username":       utils.GetUserFromRequest(r),
		},
		Query: map[config.RuleKey]string{
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
	ipsource, ipfwd := utils.GetSourceFromRequest(r)

	return &InfluxParams{
		Header: map[config.RuleKey]string{
			"authorization":  r.Header.Get("Authorization"),
			"remote-address": ipsource,
			"fwd-address":    ipfwd,
			"referer":        r.Referer(),
			"user-agent":     r.UserAgent(),
			"username":       utils.GetUserFromRequest(r),
		},
		Query: map[config.RuleKey]string{
			"db":          queryParams.Get("db"),
			"precision":   queryParams.Get("precision"),
			"consistency": queryParams.Get("consistency"),
			"rp":          queryParams.Get("rp"),
			"u":           queryParams.Get("u"),
			"p":           queryParams.Get("p"),
		},
	}
}

//REVIEW IF NEEDED !!!
func ReMapRequest(r *http.Request, params *InfluxParams, path string) *http.Request {

	//remap only Query parameters
	values := r.URL.Query()
	for qpName, qpValue := range params.Query {

		if len(qpValue) > 0 {
			//check if already exist
			existing := values.Get(string(qpName))
			if len(existing) > 0 {
				// if exist delete
				values.Del(string(qpName))
			}
			values.Add(string(qpName), qpValue)
		}
	}
	r.URL.RawQuery = values.Encode()
	if len(path) > 0 {
		r.Header.Add("X-SmartRelay-Path", path)
	}
	return r
}
