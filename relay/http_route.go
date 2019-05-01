package relay

import (
	//"errors"
	"github.com/rs/zerolog"

	"github.com/toni-moreno/influxdb-srelay/config"
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
	log     *zerolog.Logger
	filter  *regexp.Regexp
	Process func(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams)
}

func (rr *RouteRule) ActionRoute(w http.ResponseWriter, r *http.Request, start time.Time, params *InfluxParams) {

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
		return
	}

	/*match:=rr.filter.FindStringSubmatch(val)
	  if len(match) == 0 {
	    // not matched
	    rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not Matched ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)
	    return
	  }*/

	match := rr.filter.MatchString(val)
	if !match {
		rr.log.Debug().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | not Matched ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)
		return
	}
	rr.log.Info().Msgf("ROUTE RULE (ACTION: %s): Key %s | Match %s | Value %s | Matched !!!! ", rr.cfg.Name, rr.cfg.Key, rr.cfg.Match, rr.cfg.Value)

	if rr.cfg.Value == "__sinc__" {
		rr.log.Info().Msg("Database to Sinc...")
		return
	}

	if val, ok := clusters[rr.cfg.Value]; ok {
		//cluster found
		ReMapRequest(r, params, "notraced")
		rr.log.Debug().Msgf("REMAPREQUEST PRE VALUES %+v", r.URL.Query())
		if rr.Type == "WR" {
			rr.log.Info().Msg("Handle Write.....")
			val.Write(w, r, start)
			return
		}
		rr.log.Info().Msg("Handle Query....")
		val.Query(w, r, start)

	} else {
		rr.log.Warn().Msgf("There is no registered cluster %s ", rr.cfg.Value)
	}
	return

}

func (rr *RouteRule) ActionPass(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) {

}

func NewRouteRule(cfg *config.Rule, mode string, l *zerolog.Logger) (*RouteRule, error) {
	rr := &RouteRule{Type: mode, log: l}
	rr.cfg = cfg
	filter, err := regexp.Compile(cfg.Match)
	if err != nil {
		return rr, err
	}
	rr.filter = filter
	switch cfg.Action {
	case "route":
		rr.Process = rr.ActionRoute
	case "pass":
		rr.Process = rr.ActionPass
	}

	return rr, nil
}

/*func (rr *RouteRule) SetLogger(l *zerolog.Logger) {
	l.Debug().Msgf("Setting log to RouteRule : %s", rr.cfg.Name)
	rr.log = l
}*/

type HTTPRoute struct {
	cfg     *config.Route
	Type    string
	log     *zerolog.Logger
	filters []*RouteFilter
	rules   []*RouteRule
}

func NewHTTPRoute(cfg *config.Route, mode string, l *zerolog.Logger) (*HTTPRoute, error) {
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
		rr, err := NewRouteRule(r, rt.Type, rt.log)
		if err != nil {
			return rt, err
		}
		//rr.SetLogger(rt.log)
		rt.rules = append(rt.rules, rr)
	}

	return rt, nil
}

/*func (rt *HTTPRoute) SetLogger(l *zerolog.Logger) {
	l.Debug().Msgf("Setting log to Route : %s", rt.cfg.Name)
	rt.log = l
}*/

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

func (rt *HTTPRoute) ProcessRules(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) {
	for _, rule := range rt.rules {
		rule.Process(w, r, start, p)
	}
}
