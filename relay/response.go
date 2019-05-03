package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type response struct {
	code int
	body interface{}
}

type responseData struct {
	serverid        string
	clusterid       string
	location        string
	ContentType     string
	ContentEncoding string
	StatusCode      int
	Body            []byte
}

func (rd *responseData) Write(w http.ResponseWriter) {
	if rd.ContentType != "" {
		w.Header().Set("Content-Type", rd.ContentType)
	}

	if rd.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", rd.ContentEncoding)
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(rd.Body)))
	w.WriteHeader(rd.StatusCode)
	w.Write(rd.Body)
}

func jsonResponse(w http.ResponseWriter, r response) {
	data, err := json.Marshal(r.body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(r.code)

	_, _ = w.Write(data)
}

type RequestResponses struct {
	responses    []*responseData
	numResponses int
}

func InitRequesetResponse(r *http.Request) *http.Request {
	rr := &RequestResponses{responses: []*responseData{}}
	ctx := context.WithValue(r.Context(), "request_responses", rr)
	return r.WithContext(ctx)
}

func GetResponses(r *http.Request) []*responseData {
	rr := r.Context().Value("request_responses").(*RequestResponses)
	if rr != nil {
		return rr.responses
	}
	return nil
}
func GetRequestResponses(r *http.Request) *RequestResponses {
	rr := r.Context().Value("request_responses").(*RequestResponses)
	return rr
}

func (rd *responseData) AppendToRequest(r *http.Request) {
	rr := r.Context().Value("request_responses").(*RequestResponses)
	rr.responses = append(rr.responses, rd)
	rr.numResponses++
}
