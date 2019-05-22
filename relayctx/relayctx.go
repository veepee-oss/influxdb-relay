package relayctx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/toni-moreno/influxdb-srelay/backend"
	"net/http"
)

type RelayRequestCtx struct {
	//input data from query
	in            map[string]string
	Endpoint      string
	RequestSize   int
	RequestPoints int
	//Route/Rule/Cluster/Mapping
	TraceRoute bytes.Buffer
	//Responses
	responses    []*backend.ResponseData
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
	rc := &RelayRequestCtx{responses: []*backend.ResponseData{}, in: val}
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

func GetResponses(r *http.Request) []*backend.ResponseData {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	if rc != nil {
		return rc.responses
	}
	return nil
}

func AppendToRequest(r *http.Request, rd *backend.ResponseData) {
	rc := r.Context().Value("RelayRequestCtx").(*RelayRequestCtx)
	rc.responses = append(rc.responses, rd)
	rc.numResponses++
}

func JsonResponse(w http.ResponseWriter, R *http.Request, code int, body interface{}) {
	data, err := json.MarshalIndent(body, "", "\t")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SetCtxRequestSentParams(R, code, len(data))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(code)

	_, _ = w.Write(data)

}

func VoidResponse(w http.ResponseWriter, R *http.Request, code int) {
	SetCtxRequestSentParams(R, code, 0)
	w.WriteHeader(code)
}

func WriteResponse(w http.ResponseWriter, R *http.Request, resp *http.Response) {
	SetCtxRequestSentParams(R, resp.StatusCode, int(resp.ContentLength))
	resp.Write(w)
}
