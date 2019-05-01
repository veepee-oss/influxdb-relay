package relay

import (
	"compress/gzip"
	"net/http"
	"time"
)

func allMiddlewares(h *HTTP, handlerFunc relayHandlerFunc) relayHandlerFunc {
	var res = handlerFunc
	for _, middleware := range middlewares {
		res = middleware(h, res)
	}
	return res
}

func (h *HTTP) bodyMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request, start time.Time) {
		h.log.Debug().Msg("----------------------INIT bodyMiddleWare------------------------")
		var body = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			b, err := gzip.NewReader(r.Body)
			if err != nil {
				jsonResponse(w, response{http.StatusBadRequest, "unable to decode gzip body"})
				return
			}
			defer b.Close()
			body = b
		}

		r.Body = body
		next(h, w, r, start)
		h.log.Debug().Msg("----------------------END bodyMiddleWare------------------------")
	})
}

func (h *HTTP) queryMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request, start time.Time) {
		h.log.Debug().Msg("----------------------INIT queryMiddleWare------------------------")
		queryParams := r.URL.Query()

		if queryParams.Get("db") == "" && (r.URL.Path == "/write" || r.URL.Path == "/api/v1/prom/write") {
			jsonResponse(w, response{http.StatusBadRequest, "missing parameter: db"})
			return
		}

		if queryParams.Get("rp") == "" && h.rp != "" {
			queryParams.Set("rp", h.rp)
		}

		r.URL.RawQuery = queryParams.Encode()
		next(h, w, r, start)
		h.log.Debug().Msg("----------------------END queryMiddleWare------------------------")
	})

}

func (h *HTTP) logMiddleWare(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request, start time.Time) {
		h.log.Debug().Msg("----------------------INIT logMiddleWare------------------------")
		next(h, w, r, start)
		h.log.Info().Msgf("got request on: %s  from :%s %s Response %s", r.URL.Path, r.RemoteAddr, r.Referer(), time.Since(start))
		h.log.Debug().Msg("----------------------END logMiddleWare------------------------")
	})
}

func (h *HTTP) rateMiddleware(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request, start time.Time) {
		h.log.Debug().Msg("----------------------INIT rateMiddleware-----------------------")
		if h.rateLimiter != nil && !h.rateLimiter.Allow() {
			h.log.Debug().Msgf("Rate Limited => Too Many Request (Limit %+v)(Burst %d) ", h.rateLimiter.Limit(), h.rateLimiter.Burst)
			jsonResponse(w, response{http.StatusTooManyRequests, http.StatusText(http.StatusTooManyRequests)})
			return
		}

		next(h, w, r, start)
		h.log.Debug().Msg("----------------------END rateMiddleware-----------------------")
	})
}
