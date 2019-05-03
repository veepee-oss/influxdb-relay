package relay

import (
	"bytes"
	"errors"
	"github.com/rs/zerolog"

	"encoding/json"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/influxdata/influxdb/models"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/prometheus"
	"github.com/toni-moreno/influxdb-srelay/prometheus/remote"
	"net/http"
	"regexp"
	"time"
)

type RouteFilter struct {
	cfg    *config.Filter
	filter *regexp.Regexp
	log    *zerolog.Logger
}

func NewRouteFilter(cfg *config.Filter, l *zerolog.Logger) (*RouteFilter, error) {
	rf := &RouteFilter{log: l}
	rf.cfg = cfg
	filter, err := regexp.Compile(cfg.Match)
	if err != nil {
		return rf, err
	}
	rf.filter = filter
	return rf, nil
}

func (rf *RouteFilter) Match(params *InfluxParams) bool {

	val, ok := params.Header[rf.cfg.Key]
	if ok {
		rf.log.Debug().Msgf("ROUTE FILTER: Key %s, Val %s ", rf.cfg.Key, val)
		return rf.filter.MatchString(val)
	}
	val, ok = params.Query[rf.cfg.Key]
	if ok {
		rf.log.Debug().Msgf("ROUTE FILTER: Key %s, Val %s ", rf.cfg.Key, val)
		return rf.filter.MatchString(val)
	}
	rf.log.Debug().Msgf("ROUTE FILTER: Key %s not in  Params %+v ", rf.cfg.Key, params)
	return false
}

type RouteRule struct {
	cfg     *config.Rule
	Type    string
	Level   string
	log     *zerolog.Logger
	filter  *regexp.Regexp
	Process func(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) []*responseData
}

func (rr *RouteRule) ActionRouteHTTP(w http.ResponseWriter, r *http.Request, start time.Time, params *InfluxParams) []*responseData {

	val := ""
	found := false

	valH, okH := params.Header[rr.cfg.Key]
	valQ, okQ := params.Query[rr.cfg.Key]

	if okH {
		found = true
		val = valH
	}
	if okQ {
		found = true
		val = valQ
	}
	if !found {
		rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not in  Params %+v ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value, params)
		return nil
	}

	match := rr.filter.MatchString(val)
	if !match {
		rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not Matched ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)
		return nil
	}
	rr.log.Info().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | Matched !!!! ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)

	if rr.cfg.Value == "__sinc__" {
		rr.log.Info().Msg("Database to Sinc...")
		return nil
	}

	if val, ok := clusters[rr.cfg.Value]; ok {
		//cluster found
		ReMapRequest(r, params, "notraced")
		rr.log.Debug().Msgf("REMAPREQUEST PRE VALUES %+v", r.URL.Query())
		if rr.Type == "WR" {
			rr.log.Info().Msg("Handle Write.....")
			return val.WriteHTTP(w, r, start)
		}
		rr.log.Info().Msg("Handle Query....")
		val.QueryHTTP(w, r, start)
		return nil

	} else {
		rr.log.Warn().Msgf("There is no registered cluster %s ", rr.cfg.Value)
	}
	return nil

}

func (rr *RouteRule) ActionRouteData(w http.ResponseWriter, r *http.Request, start time.Time, params *InfluxParams) []*responseData {

	val := ""
	found := false

	valH, okH := params.Header[rr.cfg.Key]
	valQ, okQ := params.Query[rr.cfg.Key]

	if okH {
		found = true
		val = valH
	}
	if okQ {
		found = true
		val = valQ
	}
	if !found {
		rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not in  Params %+v ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value, params)
		return nil
	}

	match := rr.filter.MatchString(val)
	if !match {
		rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not Matched ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)
		return nil
	}
	rr.log.Info().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | Matched !!!! ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)

	if rr.cfg.Value == "__sinc__" {
		rr.log.Info().Msg("Database to Sinc...")
		return nil
	}

	if val, ok := clusters[rr.cfg.Value]; ok {

		data, err := InfluxEncode(params.Points)
		if err != nil {
			rr.log.Warn().Msgf("Data Enconding Error: %s", err)
		}

		newParams := params.Clone()

		if rr.Type == "WR" {
			rr.log.Info().Msg("Handle Write.....")
			return val.WriteData(w, newParams, data)
		}
		rr.log.Info().Msg("Handle Query....")
		val.QueryHTTP(w, r, start)
		return nil

	} else {
		rr.log.Warn().Msgf("There is no registered cluster %s ", rr.cfg.Value)
	}
	return nil

}

func (rr *RouteRule) ActionPass(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) []*responseData {
	return nil
}

// RENAME DATA PARAMETERS
func (rr *RouteRule) ActionRenameData(w http.ResponseWriter, r *http.Request, start time.Time, params *InfluxParams) []*responseData {

	switch rr.cfg.Key {
	case "measurement":
		for _, p := range params.Points {
			if rr.filter.Match(p.Name()) {
				rr.log.Debug().Msgf("Replace Measurement name %s by %s ", p.Name(), rr.cfg.Value)
				newName := rr.filter.ReplaceAllLiteralString(string(p.Name()), rr.cfg.Value)
				p.SetName(newName)
			}
		}
	case "field":
		rr.log.Warn().Msgf("Field Rename Not Supported Yet on rule %s", rr.cfg.Name)
	case "tag":
		rr.log.Warn().Msgf("Tag Rename Not Supported Yet on rule %s", rr.cfg.Name)
	default:
		rr.log.Warn().Msgf("Rename Data key  %d not supported in rule %s", rr.cfg.Key, rr.cfg.Name)
	}
	return nil
}

// RENAME Influx
func (rr *RouteRule) ActionRenameDataHTTP(w http.ResponseWriter, r *http.Request, start time.Time, params *InfluxParams) []*responseData {
	return nil
}

func NewRouteRule(cfg *config.Rule, mode string, l *zerolog.Logger, routelevel string) (*RouteRule, error) {
	rr := &RouteRule{Type: mode, log: l}
	rr.cfg = cfg
	filter, err := regexp.Compile(cfg.Match)
	if err != nil {
		return rr, err
	}
	rr.filter = filter
	rr.Level = routelevel
	switch cfg.Action {
	case "route":
		switch routelevel {
		case "http":
			rr.Process = rr.ActionRouteHTTP
		case "data":
			rr.Process = rr.ActionRouteData
		default:
		}
	case "pass":
		rr.Process = rr.ActionPass
	case "rename_data":
		rr.Process = rr.ActionRenameData
	case "rename_data_http":
		rr.Process = rr.ActionRenameDataHTTP
	default:
		return rr, errors.New("Unknown rule action " + cfg.Action + " on Rule:" + rr.cfg.Name)
	}

	return rr, nil

}

func (rt *HTTPRoute) DecodePrometheus(w http.ResponseWriter, r *http.Request) (models.Points, error) {

	var bodyBuf bytes.Buffer
	_, err := bodyBuf.ReadFrom(r.Body)
	if err != nil {
		return nil, err
	}

	reqBuf, err := snappy.Decode(nil, bodyBuf.Bytes())
	if err != nil {
		rt.log.Error().Msgf("Error on snappy decode prometheus : %s", err)
		return nil, err
	}

	// Convert the Prometheus remote write request to Influx Points
	var req remote.WriteRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		rt.log.Error().Msgf("Error on Unmarshall decode Prometheus ")
		return nil, err
	}
	points, err := prometheus.WriteRequestToPoints(&req)
	if err != nil {

		//c.log.Printf("Prom write handler Error %s", err)

		// Check if the error was from something other than dropping invalid values.
		if _, ok := err.(prometheus.DroppedValuesError); !ok {
			//c.httpError(w, err.Error(), http.StatusBadRequest)
			return points, err
		}
	}
	return points, nil
}

func (rt *HTTPRoute) DecodeInflux(w http.ResponseWriter, r *http.Request) (models.Points, error) {

	var bodyBuf bytes.Buffer
	_, err := bodyBuf.ReadFrom(r.Body)
	if err != nil {
		return nil, err
	}
	queryParams := r.URL.Query()
	precision := queryParams.Get("precision")
	//not sure tu use models.ParsePointsWithPrecision ( or perhaps betther models.ParsePoints)
	points, err := models.ParsePointsWithPrecision(bodyBuf.Bytes(), time.Now(), precision)
	if err != nil {
		rt.log.Error().Msgf("parse points error: %s", err)
		return nil, errors.New("Unable to parse points: " + err.Error())
	}

	return points, nil
}

func InfluxEncodePrecision(points models.Points, precision string) (*bytes.Buffer, error) {

	var output bytes.Buffer
	var err error
	for _, p := range points {
		// Those two functions never return any errors, let's just ignore the return value
		_, err = output.WriteString(p.PrecisionString(precision))
		if err != nil {
			return &output, err
		}
		err = output.WriteByte('\n')
		if err != nil {
			return &output, err
		}
	}
	return &output, nil
}

func InfluxEncode(points models.Points) (*bytes.Buffer, error) {

	var output bytes.Buffer
	var err error
	for _, p := range points {
		// Those two functions never return any errors, let's just ignore the return value
		_, err = output.WriteString(p.String())
		if err != nil {
			return &output, err
		}
		err = output.WriteByte('\n')
		if err != nil {
			return &output, err
		}
	}
	return &output, nil
}

type HTTPRoute struct {
	cfg        *config.Route
	Type       string
	log        *zerolog.Logger
	filters    []*RouteFilter
	rules      []*RouteRule
	DecodeData func(w http.ResponseWriter, r *http.Request) (models.Points, error)
	responses  []*responseData
}

func NewHTTPRoute(cfg *config.Route, mode string, l *zerolog.Logger, format string) (*HTTPRoute, error) {
	rt := &HTTPRoute{Type: mode, log: l}

	rt.cfg = cfg
	for _, f := range cfg.Filter {
		rf, err := NewRouteFilter(f, rt.log)
		if err != nil {
			return rt, err
		}
		//rf.SetLogger(rt.log)
		rt.filters = append(rt.filters, rf)
	}
	for _, r := range cfg.Rule {
		rr, err := NewRouteRule(r, rt.Type, rt.log, rt.cfg.Level)
		if err != nil {
			return rt, err
		}
		//rr.SetLogger(rt.log)
		rt.rules = append(rt.rules, rr)
	}

	if cfg.Level == "data" {
		switch format {
		case "prom-write":
			rt.DecodeData = rt.DecodePrometheus
		case "ILP":
			rt.DecodeData = rt.DecodeInflux
		}
	}

	return rt, nil
}

// return true if any of its condition match
// c1 or c2 or c3 or c4
func (rt *HTTPRoute) MatchFilter(params *InfluxParams) bool {
	//return tr
	for _, f := range rt.filters {
		if f.Match(params) {
			return true
		}
	}
	return false
}

// TODO: build an only httpError interface for all
func httpError(w http.ResponseWriter, errmsg string, code int) {
	// Default implementation if the response writer hasn't been replaced
	// with our special response writer type.
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(errmsg)
	w.Write(b)
}

func (rt *HTTPRoute) ProcessRules(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) {

	if rt.cfg.Level == "data" {
		points, err := rt.DecodeData(w, r)
		if err != nil {
			rt.log.Error().Msgf("Error in Rule %s when decoding data : %s", rt.cfg.Name, err)
			httpError(w, err.Error(), http.StatusBadRequest)
			return
		}

		p.Points = points
	}

	for _, rule := range rt.rules {
		resp := rule.Process(w, r, start, p)
		rt.responses = append(rt.responses, resp...)
	}
}
