package relay

import (
	"net/http"
	"time"
)

func (h *HTTP) handleAdmin(w http.ResponseWriter, r *http.Request, _ time.Time) {
	/*
		// Client to perform the raw queries
		client := http.Client{}

		// Base body for all requests
		baseBody := bytes.Buffer{}
		_, err := baseBody.ReadFrom(r.Body)
		if err != nil {
			log.Printf("relay %q: could not read body: %v", h.Name(), err)
			return
		}

		if r.Method != http.MethodPost {
			// Bad method
			w.Header().Set("Allow", http.MethodPost)
			backend.JsonResponse(w, response{http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)})
			return
		}

		// Responses
		var responses = make(chan *http.Response, len(h.backends))

		// Associated waitgroup
		var wg sync.WaitGroup
		wg.Add(len(h.backends))

		// Iterate over all backends
		for _, b := range h.backends {
			b := b

			go func() {
				defer wg.Done()

				bodyBytes := baseBody

				// Create new request
				// Update location according to backend
				// Forward body
				req, err := http.NewRequest("POST", b.location+b.endpoints.Query, &bodyBytes)
				if err != nil {
					log.Printf("problem posting to relay %q backend %q: could not prepare request: %v", h.Name(), b.name, err)
					responses <- &http.Response{}
					return
				}

				// Forward headers
				req.Header = r.Header

				// Forward the request
				resp, err := client.Do(req)
				if err != nil {
					// Internal error
					log.Printf("problem posting to relay %q backend %q: %v", h.Name(), b.name, err)

					// So empty response
					responses <- &http.Response{}
				} else {
					if resp.StatusCode/100 == 5 {
						// HTTP error
						log.Printf("5xx response for relay %q backend %q: %v", h.Name(), b.name, resp.StatusCode)
					}

					// Get response
					responses <- resp
				}
			}()
		}

		// Wait for requests
		go func() {
			wg.Wait()
			close(responses)
		}()

		var errResponse *ResponseData
		for resp := range responses {
			switch resp.StatusCode / 100 {
			case 2:
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
			backend.JsonResponse(w, response{http.StatusServiceUnavailable, "unable to forward query"})
			return
		}
	*/
	h.log.Info().Msg("Admin Feature not enabled yet")
}
