package relay

import (
	"compress/gzip"

	"net/http"

	"time"

	"github.com/toni-moreno/influxdb-srelay/pkg/relayctx"
	"github.com/toni-moreno/influxdb-srelay/pkg/utils"
)

func allMiddlewares(h *HTTP, handlerFunc relayHandlerFunc) relayHandlerFunc {
	var res = handlerFunc
	for _, middleware := range middlewares {
		res = middleware(h, res)
	}
	return res
}

func (h *HTTP) bodyMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request) {
		h.log.Debug().Msg("----------------------INIT bodyMiddleWare------------------------")
		var body = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			b, err := gzip.NewReader(r.Body)
			if err != nil {
				relayctx.JsonResponse(w, r, http.StatusBadRequest, "unable to decode gzip body")
				return
			}
			defer b.Close()
			body = b
		}

		r.Body = body
		next(h, w, r)
		h.log.Debug().Msg("----------------------END bodyMiddleWare------------------------")
	})
}

func (h *HTTP) queryMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request) {
		h.log.Debug().Msg("----------------------INIT queryMiddleWare------------------------")
		queryParams := r.URL.Query()

		if queryParams.Get("db") == "" && (r.URL.Path == "/write" || r.URL.Path == "/api/v1/prom/write") {
			relayctx.JsonResponse(w, r, http.StatusBadRequest, "missing parameter: db")
			return
		}

		if queryParams.Get("rp") == "" && h.rp != "" {
			queryParams.Set("rp", h.rp)
		}

		r.URL.RawQuery = queryParams.Encode()
		next(h, w, r)
		h.log.Debug().Msg("----------------------END queryMiddleWare------------------------")
	})

}

func (h *HTTP) logMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request) {
		h.log.Debug().Msg("----------------------INIT logMiddleWare------------------------")
		next(h, w, r)
		rc := relayctx.GetRelayContext(r)
		if rc.Served {
			_, ipfwd := utils.GetSourceFromRequest(r)
			h.acclog.Info().
				Str("trace-route", rc.TraceRoute.String()).
				Str("referer", r.Referer()).
				Str("url", r.URL.String()).
				Int("write-size", rc.RequestSize).
				Int("write-points", rc.RequestPoints).
				Int("returnsize", rc.SentDataLength).
				Dur("duration_ms", time.Since(rc.InputTime)).
				Dur("bk_duration_ms", time.Since(rc.BackendTime)).
				Dur("latency_ms", rc.BackendTime.Sub(rc.InputTime)).
				Int("status", rc.SentHTTPStatus).
				Str("method", r.Method).
				Str("user", utils.GetUserFromRequest(r)). // <---allready computed from http_params !! REVIEW!!!
				Str("source", ipfwd).                     // Here log all forwarders
				Str("user-agent", r.UserAgent()).
				Msg("")
		}

		h.log.Debug().Msg("----------------------END logMiddleWare------------------------")
	})
}

func (h *HTTP) rateMiddleware(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request) {
		h.log.Debug().Msg("----------------------INIT rateMiddleware-----------------------")
		if h.rateLimiter != nil && !h.rateLimiter.Allow() {
			h.log.Debug().Msgf("Rate Limited => Too Many Request (Limit %+v)(Burst %d) ", h.rateLimiter.Limit(), h.rateLimiter.Burst)
			relayctx.JsonResponse(w, r, http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests))
			return
		}

		next(h, w, r)
		h.log.Debug().Msg("----------------------END rateMiddleware-----------------------")
	})
}
