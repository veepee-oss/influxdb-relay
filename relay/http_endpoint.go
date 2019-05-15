package relay

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayctx"
)

type HTTPEndPoint struct {
	cfg         *config.Endpoint
	log         *zerolog.Logger
	routes      []*HTTPRoute
	process     func(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams)
	splitParams func(r *http.Request) *backend.InfluxParams
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
		e.splitParams = backend.SplitParamsIQL
	case "ILP":
		e.splitParams = backend.SplitParamsILP
	case "prom-write":
		e.splitParams = backend.SplitParamsPRW
	default:
		return e, errors.New("Unknown Source Format " + e.cfg.SourceFormat)
	}
	for _, r := range e.cfg.Route {
		rt, err := NewHTTPRoute(r, cfg.Type, e.log, cfg.SourceFormat)
		if err != nil {
			return e, err
		}
		e.routes = append(e.routes, rt)
	}

	return e, nil
}

func (e *HTTPEndPoint) ProcessRead(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams) {
	//AppendCxtTracePath(r, "endp|READ", e.cfg.URI[0])
	processed := false
	for k, router := range e.routes {
		e.log.Debug().Msgf("Processing READ route %d , %s", k, router.cfg.Name)
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

func (e *HTTPEndPoint) ProcessWrite(w http.ResponseWriter, r *http.Request, start time.Time, p *backend.InfluxParams) {
	//AppendCxtTracePath(r, "endp|WRITE", e.cfg.URI[0])
	processed := false
	for k, router := range e.routes {
		e.log.Debug().Msgf("Processing WRITE route %d , %s", k, router.cfg.Name)
		match := router.MatchFilter(p)
		if match {
			e.log.Debug().Msgf("Route %s Match!!!!", router.cfg.Name)
			e.log.Debug().Msgf("Processing WRITE route %d , %+v", k, router)
			processed = true
			relayctx.AppendCxtTracePath(r, "rt", router.cfg.Name)
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
			relayctx.SetCtxEndpoint(r, uri)
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
