package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"regexp"
	"time"

	"github.com/rs/zerolog"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/influxdata/influxdb/models"

	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayctx"
	"github.com/toni-moreno/influxdb-srelay/utils"

	"github.com/toni-moreno/influxdb-srelay/prometheus"
	"github.com/toni-moreno/influxdb-srelay/prometheus/remote"
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

func (rf *RouteFilter) Match(params *backend.InfluxParams) bool {

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
	Type    config.EndPType
	Level   config.RouteLevel
	log     *zerolog.Logger
	filter  *regexp.Regexp
	Process func(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams) []*backend.ResponseData
}

func (rr *RouteRule) ActionRouteHTTP(w http.ResponseWriter, r *http.Request, start time.Time, params *backend.InfluxParams) []*backend.ResponseData {

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

	if rr.cfg.ToCluster == "__sinc__" {
		rr.log.Info().Msg("Database to Sinc...")
		return nil
	}

	if val, ok := clusters[rr.cfg.ToCluster]; ok {
		//cluster found
		backend.ReMapRequest(r, params, "notraced")
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

func (rr *RouteRule) ActionRouteData(w http.ResponseWriter, r *http.Request, start time.Time, params *backend.InfluxParams) []*backend.ResponseData {

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

	if val, ok := clusters[rr.cfg.ToCluster]; ok {

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

func (rr *RouteRule) ActionPass(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams) []*backend.ResponseData {
	return nil
}

// RENAME DATA PARAMETERS
func (rr *RouteRule) ActionRenameData(w http.ResponseWriter, r *http.Request, start time.Time, params *backend.InfluxParams) []*backend.ResponseData {

	switch rr.cfg.Key {
	case "measurement":
		for _, p := range params.Points {
			if rr.filter.Match(p.Name()) {
				rr.log.Debug().Msgf("Replace Measurement name %s by %s ", p.Name(), rr.cfg.Value)
				newName := rr.filter.ReplaceAllString(string(p.Name()), rr.cfg.Value)
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
func (rr *RouteRule) ActionRenameDataHTTP(w http.ResponseWriter, r *http.Request, start time.Time, params *backend.InfluxParams) []*backend.ResponseData {
	return nil
}

func (rr *RouteRule) ActionRouteDBFromData(w http.ResponseWriter, r *http.Request, start time.Time, params *backend.InfluxParams) []*backend.ResponseData {

	if rr.Type != "WR" {
		rr.log.Error().Msgf("Error Wrong type Rule in %s", rr.cfg.Name)
		return nil
	}
	dbs := make(map[string]models.Points)

	switch rr.cfg.Key {
	case "measurement":
		for _, p := range params.Points {
			if rr.filter.Match(p.Name()) {
				dbName := rr.filter.ReplaceAllString(string(p.Name()), rr.cfg.Value)
				rr.log.Debug().Msgf("Got DB name %s from  Measurement name %s by %s ", dbName, p.Name(), rr.cfg.Value)
				if newpoints, ok := dbs[dbName]; ok {
					newpoints = append(newpoints, p)
					dbs[dbName] = newpoints
				} else {
					dbs[dbName] = models.Points{p}
				}
			}
		}
	case "tag":
		tagkey := rr.cfg.KeyAux
		// need for rr.cfg.Key_aux
		for _, p := range params.Points {
			//rr.log.Debug().Msgf("POINT :%+v", p)
			var tagvalue []byte
			for _, t := range p.Tags() {
				//				rr.log.Debug().Msgf("Found Tag %d [%s] tagkey[%s] with value [%s]  | %s", k, t.Key, tagkey, t.Value, t.String())
				if string(t.Key) == tagkey {
					tagvalue = t.Value
					break
				}
			}

			if len(tagvalue) > 0 && rr.filter.Match(tagvalue) {
				//rr.log.Debug().Msgf("Found Tag key [%s] with value [%s]", tagkey, tagvalue)
				dbName := rr.filter.ReplaceAllString(string(tagvalue), rr.cfg.Value)
				rr.log.Debug().Msgf("Got DB name %s from Tag key %s Tag Value %s", dbName, tagkey, tagvalue)
				//rr.log.Debug().Msgf("POINT :%+v", p)
				if newpoints, ok := dbs[dbName]; ok {
					newpoints = append(newpoints, p)
					dbs[dbName] = newpoints
				} else {
					dbs[dbName] = models.Points{p}
				}

			} else {
				//not match or not found the tag
				if len(rr.cfg.ValueOnUnMatch) > 0 {
					dbName := rr.cfg.ValueOnUnMatch
					if newpoints, ok := dbs[dbName]; ok {
						newpoints = append(newpoints, p)
						dbs[dbName] = newpoints
					} else {
						dbs[dbName] = models.Points{p}
					}
				}
				//not match
				rr.log.Debug().Msgf("Point does not match namespace TAGVALUE %s (Measuerement : %s) TAGS %+v", tagvalue, p.Name(), p.Tags())
			}
		}

	case "field":
		rr.log.Warn().Msgf("Field Based Route Not Supported Yet on rule %s", rr.cfg.Name)

	default:
		rr.log.Warn().Msgf("Rename Data key  %d not supported in rule %s", rr.cfg.Key, rr.cfg.Name)
	}

	for db, p := range dbs {

		rr.log.Info().Msgf("processing output for db %s : # %d Points", db, len(p))
		//rr.log.Debug().Msgf("processing output for db %s : %+v", db, p)

		if val, ok := clusters[rr.cfg.ToCluster]; ok {

			data, err := InfluxEncode(p)
			if err != nil {
				rr.log.Warn().Msgf("Data Enconding Error: %s", err)
				continue
			}

			newParams := params.Clone()
			newParams.SetDB(db)

			rr.log.Info().Msgf("Handle DB route Write to %s.....", db)
			resp := val.WriteData(w, newParams, data)
			for _, r := range resp {
				if r.StatusCode/100 != 2 {
					rr.log.Error().Msgf("Error in write data to %s : Error: %s ", r.Serverid, string(r.Body))
				}
			}

		} else {
			rr.log.Warn().Msgf("There is no registered cluster %s ", rr.cfg.Value)
		}

	}
	return nil
}

func NewRouteRule(cfg *config.Rule, mode config.EndPType, l *zerolog.Logger, routelevel config.RouteLevel) (*RouteRule, error) {
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
	case "route_db_from_data":
		rr.Process = rr.ActionRouteDBFromData
	default:
		return rr, fmt.Errorf("Unknown rule action  %s  on Rule: %s", cfg.Action, rr.cfg.Name)
	}

	return rr, nil

}

func (rt *HTTPRoute) DecodePrometheus(w http.ResponseWriter, r *http.Request) (int, models.Points, error) {

	var bodyBuf bytes.Buffer
	_, err := bodyBuf.ReadFrom(r.Body)
	if err != nil {
		return 0, nil, err
	}

	reqBuf, err := snappy.Decode(nil, bodyBuf.Bytes())
	if err != nil {
		rt.log.Error().Msgf("Error on snappy decode prometheus : %s", err)
		return 0, nil, err
	}

	// Convert the Prometheus remote write request to Influx Points
	var req remote.WriteRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		rt.log.Error().Msgf("Error on Unmarshall decode Prometheus ")
		return 0, nil, err
	}
	points, err := prometheus.WriteRequestToPoints(&req)
	if err != nil {

		//c.log.Printf("Prom write handler Error %s", err)

		// Check if the error was from something other than dropping invalid values.
		if _, ok := err.(prometheus.DroppedValuesError); !ok {
			//c.httpError(w, err.Error(), http.StatusBadRequest)
			return len(reqBuf), points, err
		}
	}
	return len(reqBuf), points, nil
}

func (rt *HTTPRoute) DecodeInflux(w http.ResponseWriter, r *http.Request) (int, models.Points, error) {

	var bodyBuf bytes.Buffer
	_, err := bodyBuf.ReadFrom(r.Body)
	if err != nil {
		return 0, nil, err
	}
	queryParams := r.URL.Query()
	precision := queryParams.Get("precision")
	//not sure tu use models.ParsePointsWithPrecision ( or perhaps betther models.ParsePoints)
	points, err := models.ParsePointsWithPrecision(bodyBuf.Bytes(), time.Now(), precision)
	if err != nil {
		rt.log.Error().Msgf("parse points error: %s", err)
		return 0, nil, errors.New("Unable to parse points: " + err.Error())
	}

	return bodyBuf.Len(), points, nil
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
	DecodeFmt  config.EndPSFormat
	Type       config.EndPType
	log        *zerolog.Logger
	filters    []*RouteFilter
	rules      []*RouteRule
	DecodeData func(w http.ResponseWriter, r *http.Request) (int, models.Points, error)
}

func NewHTTPRoute(cfg *config.Route, mode config.EndPType, l *zerolog.Logger, format config.EndPSFormat) (*HTTPRoute, error) {
	rt := &HTTPRoute{Type: mode, log: l}

	rt.cfg = cfg

	//Log output
	if !cfg.LogInherit {
		var filename string
		if len(cfg.LogFile) > 0 {
			filename = cfg.LogFile
		} else {
			filename = "http_route_" + cfg.Name + ".log"
		}
		rt.log = utils.GetConsoleLogFormated(filename, cfg.LogLevel)
	}

	for _, f := range cfg.Filter {
		rf, err := NewRouteFilter(f, rt.log)
		if err != nil {
			return rt, err
		}
		rt.filters = append(rt.filters, rf)
	}
	for _, r := range cfg.Rule {
		rr, err := NewRouteRule(r, rt.Type, rt.log, rt.cfg.Level)
		if err != nil {
			return rt, err
		}
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
	rt.DecodeFmt = format

	return rt, nil
}

// return true if any of its condition match
// c1 or c2 or c3 or c4
func (rt *HTTPRoute) MatchFilter(params *backend.InfluxParams) bool {
	//return tr
	for _, f := range rt.filters {
		if f.Match(params) {
			return true
		}
	}
	return false
}

func (rt *HTTPRoute) HandleHTTPResponse(w http.ResponseWriter, r *http.Request) {

	var errResponse *backend.ResponseData

	responses := relayctx.GetResponses(r)

	if len(responses) == 0 {
		rt.log.Info().Msgf("No responses found on request route %s ", rt.cfg.Name)
		return
	}
	rt.log.Debug().Msgf("Recovered %d, responses", len(responses))

	w.Header().Set("Content-Type", "text/plain")

	for _, resp := range responses {
		relayctx.SetCtxRequestSentParams(r, resp.StatusCode, len(resp.Body))
		rt.log.Debug().Msgf("RESPONSE from (CLUSTER:%s|SERVER:%s|LOCATION:%s) : HTTP CODE %d content/type  (%s) %s", resp.Clusterid, resp.Serverid, resp.Location, resp.StatusCode, resp.ContentType, string(resp.Body))
		switch resp.StatusCode / 100 {
		case 2:
			// Status accepted means buffering,
			if resp.StatusCode == http.StatusAccepted {
				rt.log.Info().Msg("could not reach relay, buffering...")
				w.WriteHeader(http.StatusAccepted)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return

		case 4:
			// User error
			resp.Write(w)

			return

		default:
			// Hold on to one of the responses to return back to the client
			errResponse = nil
		}
	}

	// No successful writes
	if errResponse == nil {
		// Failed to make any valid request...
		relayctx.JsonResponse(w, r, http.StatusServiceUnavailable, "unable to write points")
		return
	}
}

// TODO: build an only httpError interface for all
func httpError(w http.ResponseWriter, r *http.Request, errmsg string, code int) {
	// Default implementation if the response writer hasn't been replaced
	// with our special response writer type.
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	b, _ := json.Marshal(errmsg)
	w.Write(b)
	relayctx.SetCtxRequestSentParams(r, code, len(b))
}

func (rt *HTTPRoute) ProcessRules(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams) {

	if rt.cfg.Level == "data" {
		relayctx.AppendCxtTracePath(r, "decode", string(rt.DecodeFmt))
		size, points, err := rt.DecodeData(w, r)
		if err != nil && points == nil {
			rt.log.Error().Msgf("Error in Rule %s when decoding data : %s", rt.cfg.Name, err)
			if points != nil {
				rt.log.Error().Msgf("ERROR POINTS  DATA %+v", points)
			}

			httpError(w, r, err.Error(), http.StatusBadRequest)
			return
		}

		p.Points = points
		relayctx.SetCtxRequestSize(r, size, len(points))
	}

	//R = r + Response handler
	//R := InitRelayContext(r)

	for _, rule := range rt.rules {
		relayctx.AppendCxtTracePath(r, "rule", rule.cfg.Name)
		responses := rule.Process(w, r, start, p)
		for _, resp := range responses {
			relayctx.AppendToRequest(r, resp)
		}

	}
	rt.HandleHTTPResponse(w, r)

}
