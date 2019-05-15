package backend

import (
	//"bytes"
	//"context"

	"net/http"
	"strconv"
)

/*type Response struct {
	code int
	body interface{}
}*/

type ResponseData struct {
	Serverid        string
	Clusterid       string
	Location        string
	ContentType     string
	ContentEncoding string
	StatusCode      int
	Body            []byte
}

func (rd *ResponseData) Write(w http.ResponseWriter) {
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

//func JsonResponse(w http.ResponseWriter, R *http.Request, r Response) {

type RequestResponses struct {
	responses    []*ResponseData
	numResponses int
}
