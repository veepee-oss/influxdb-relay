package relay

import (
	"github.com/rs/zerolog"
	//"errors"
	"fmt"
	//"github.com/influxdata/influxdb/models"
	"github.com/toni-moreno/influxdb-srelay/config"
	//"regexp"
	"time"
)

type dbBackend struct {
	cfg *config.InfluxDBBackend
	poster
	log       *zerolog.Logger
	inputType config.Input
	admin     string
	//endpoints config.HTTPEndpointConfig

	//tagRegexps         []*regexp.Regexp
	//measurementRegexps []*regexp.Regexp
}

// validateRegexps checks if a request on this backend matches
// all the tag regular expressions for this backend
/*
func (b *dbBackend) validateRegexps(ps models.Points) error {
	// For each point
	for _, p := range ps {
		// Check if the measurement of each point
		// matches ALL measurement regular expressions
		m := p.Name()
		for _, r := range b.measurementRegexps {
			if !r.Match(m) {
				return errors.New("bad measurement")
			}
		}

		// For each tag of each point
		for _, t := range p.Tags() {
			// Check if each tag of each point
			// matches ALL tags regular expressions
			for _, r := range b.tagRegexps {
				if !r.Match(t.Key) {
					return errors.New("bad tag")
				}
			}
		}
	}

	return nil
}*/

func (b *dbBackend) getRetryBuffer() *retryBuffer {
	if p, ok := b.poster.(*retryBuffer); ok {
		return p
	}
	return nil
}

func NewDBBackend(cfg *config.InfluxDBBackend, l *zerolog.Logger) (*dbBackend, error) {

	ret := &dbBackend{cfg: cfg, log: l}

	// Set a timeout
	timeout := DefaultHTTPTimeout
	if cfg.Timeout != "" {
		t, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("error parsing HTTP timeout '%v'", err)
		}
		timeout = t
	}

	// Get underlying Poster instance
	var p poster = newSimplePoster(cfg.Location, timeout, cfg.SkipTLSVerification)

	// If configured, create a retryBuffer per backend.
	// This way we serialize retries against each backend.
	if cfg.BufferSizeMB > 0 {
		max := DefaultMaxDelayInterval
		if cfg.MaxDelayInterval != "" {
			m, err := time.ParseDuration(cfg.MaxDelayInterval)
			if err != nil {
				return nil, fmt.Errorf("error parsing max retry time %v", err)
			}
			max = m
		}

		batch := DefaultBatchSizeKB * KB
		if cfg.MaxBatchKB > 0 {
			batch = cfg.MaxBatchKB * KB
		}

		p = newRetryBuffer(cfg.BufferSizeMB*MB, batch, max, p)
	}

	/*return &dbBackend{
		poster: p,
	}, nil*/
	ret.poster = p
	return ret, nil
}
