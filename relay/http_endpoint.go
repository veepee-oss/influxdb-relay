package relay

import (
	"errors"
	"github.com/rs/zerolog"
	"github.com/toni-moreno/influxdb-srelay/config"
	"net/http"
	"time"
)

type HTTPEndPoint struct {
	cfg         *config.Endpoint
	log         *zerolog.Logger
	routes      []*HTTPRoute
	process     func(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams)
	splitParams func(r *http.Request) *InfluxParams
}

func NewHTTPEndpoint(cfg *config.Endpoint, l *zerolog.Logger) (*HTTPEndPoint, error) {
	e := &HTTPEndPoint{log: l}
	e.cfg = cfg
	switch e.cfg.Type {
	case "RD":
		e.process = e.ProcessRead
	case "WR":
		e.process = e.ProcessWrite
	default:
		err := errors.New("Error on Endpoint type " + e.cfg.Type)
		e.log.Err(err)
		return e, err
	}
	switch e.cfg.SourceFormat {
	case "IQL":
		e.splitParams = SplitParamsIQL
	case "ILP":
		e.splitParams = SplitParamsILP
	case "prom-write":
		e.splitParams = SplitParamsPRW
	default:
		return e, errors.New("Unknown Source Format " + e.cfg.SourceFormat)
	}
	for _, r := range e.cfg.Route {
		rt, err := NewHTTPRoute(r, cfg.Type, e.log)
		if err != nil {
			return e, err
		}
		e.routes = append(e.routes, rt)
	}

	return e, nil
}

func (e *HTTPEndPoint) ProcessRead(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) {
	processed := false
	for k, router := range e.routes {
		e.log.Info().Msgf("Processing READ route %d , %s", k, router.cfg.Name)
		match := router.MatchFilter(p)
		if match {
			e.log.Debug().Msgf("Route %s Match!!!!", router.cfg.Name)
			e.log.Debug().Msgf("Processing READ route %d , %+v", k, router)
			processed = true
			router.ProcessRules(w, r, start, p)
			break
		}
	}
	if !processed {
		e.log.Warn().Msgf("Any Route has processed the enpoint %+v Request:  %+v", e.cfg.URI, r)
	}

}

func (e *HTTPEndPoint) ProcessWrite(w http.ResponseWriter, r *http.Request, start time.Time, p *InfluxParams) {
	processed := false
	for k, router := range e.routes {
		e.log.Info().Msgf("Processing WRITE route %d , %s", k, router.cfg.Name)
		match := router.MatchFilter(p)
		if match {
			e.log.Debug().Msgf("Route %s Match!!!!", router.cfg.Name)
			e.log.Debug().Msgf("Processing WRITE route %d , %+v", k, router)
			processed = true
			router.ProcessRules(w, r, start, p)
			break
		}
	}
	if !processed {
		e.log.Warn().Msgf("Any Route has processed the enpoint %+v Request: %+v", e.cfg.URI, r)
	}
}

func (e *HTTPEndPoint) ProcessInput(w http.ResponseWriter, r *http.Request, start time.Time) bool {
	//check if match uri's
	found := false
	uri := ""
	for _, endpointURI := range e.cfg.URI {
		if r.URL.Path == endpointURI {
			found = true
			uri = endpointURI
		}
	}
	if !found {
		e.log.Debug().Msgf("Discarding Input  Endpoint  %s for endpoints %+v", r.URL.Path, e.cfg.URI)
		return false
	}

	e.log.Info().Msgf("Init Processing Endpoint %s", uri)
	params := e.splitParams(r)
	e.process(w, r, start, params)

	return true
}
