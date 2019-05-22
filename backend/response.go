package backend

import (
	//"bytes"
	//"context"

	"encoding/json"
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

func (m *ResponseData) MarshalJSON() ([]byte, error) {
	type Alias ResponseData
	return json.Marshal(&struct {
		Body string `json:"Body"`
		*Alias
	}{
		Body:  string(m.Body),
		Alias: (*Alias)(m),
	})
}

func (m *ResponseData) UnmarshalJSON(data []byte) error {
	type Alias ResponseData
	aux := &struct {
		Body string `json:"Body"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Body = []byte(aux.Body)
	return nil
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
