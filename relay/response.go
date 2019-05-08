package relay

import (
	"bytes"
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

func jsonResponse(w http.ResponseWriter, R *http.Request, r response) {

	data, err := json.Marshal(r.body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SetCtxRequestSentParams(R, r.code, len(data))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(r.code)

	_, _ = w.Write(data)

}

type RequestResponses struct {
	responses    []*responseData
	numResponses int
}

type RelayRequestCtx struct {
	//input data from query
	in            map[string]string
	Endpoint      string
	RequestSize   int
	RequestPoints int
	//Route/Rule/Cluster/Mapping
	TraceRoute bytes.Buffer
	//Responses
	responses    []*responseData
	numResponses int
	//Final HTTP Data Sent
	SentHTTPStatus int
	SentDataLength int
}

func AppendCxtTracePath(r *http.Request, component string, path string) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		rc.TraceRoute.WriteString(component)
		rc.TraceRoute.WriteString(":")
		rc.TraceRoute.WriteString(path)
		rc.TraceRoute.WriteString("> ")
	}
}

func InitRelayContext(r *http.Request) *http.Request {
	val := make(map[string]string)
	rc := &RelayRequestCtx{responses: []*responseData{}, in: val}
	ctx := context.WithValue(r.Context(), "RelayRequestCtx", rc)
	return r.WithContext(ctx)
}

func GetRelayContext(r *http.Request) *RelayRequestCtx {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		return rc
	}
	return nil
}

func SetCtxParam(r *http.Request, key string, value string) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		rc.in[key] = value
	}
	return
}

func SetCtxRequestSentParams(r *http.Request, httpStatus int, datalength int) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		rc.SentHTTPStatus = httpStatus
		rc.SentDataLength = datalength
	}
}

func SetCtxEndpoint(r *http.Request, endp string) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		rc.Endpoint = endp
	}
}

func SetCtxRequestSize(r *http.Request, size int, numpoints int) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		rc.RequestSize = size
		rc.RequestPoints = numpoints
	}
}

func GetCtxParam(r *http.Request, key string) string {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		if val, ok := rc.in[key]; ok {
			return val
		}
	}
	return ""
}

func GetResponses(r *http.Request) []*responseData {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		return rc.responses
	}
	return nil
}

func (rd *responseData) AppendToRequest(r *http.Request) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	rc.responses = append(rc.responses, rd)
	rc.numResponses++
}
