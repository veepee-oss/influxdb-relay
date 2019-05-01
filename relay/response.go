package relay

import (
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
	id              string
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
