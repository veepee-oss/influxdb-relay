package relay

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/strike-team/influxdb-relay/config"
	"time"
)

// Body is an empty body which implements io.Reader in order to create valid http.Requests
type Body struct {
	buf *bytes.Buffer
}

func (b Body) Read(p []byte) (int, error)   { return b.buf.Read(p) }
func (b *Body) Write(p []byte) (int, error) { return b.buf.Write(p) }
func (b Body) Close() error                 { return nil }

// End is a placeholder http Handler used to test middlewares
func (h *HTTP) End(w http.ResponseWriter, r *http.Request, s time.Time) {
	return
}

// EndTag is a placeholder http Handler used to test middlewares, which changes a variable
func (h *HTTP) EndTag(w http.ResponseWriter, r *http.Request, s time.Time) {
	wasInEnd = true
	return
}

func (h *HTTP) tagMiddleware(next relayHandlerFunc) relayHandlerFunc {
	return relayHandlerFunc(func(h *HTTP, w http.ResponseWriter, r *http.Request, s time.Time) {
		wasInMiddleware = true
		next(h, w, r, s)
	})
}

// ResponseWriter is a placeholder http.ResponseWriter to give to middlewares
type ResponseWriter struct {
	writeBuf *bytes.Buffer
	code     int
	header   http.Header
}

func (r *ResponseWriter) Header() http.Header         { return r.header }
func (r *ResponseWriter) Write(p []byte) (int, error) { return r.writeBuf.Write(p) }
func (r *ResponseWriter) WriteHeader(statusCode int)  { r.code = statusCode }

type Logger struct {
	buffer *bytes.Buffer
}

func (l Logger) Write(p []byte) (n int, err error) { return l.buffer.Write(p) }

var (
	emptyBody     = Body{}
	wrongGzipBody = Body{}
	gzipBody      = Body{}

	wasInMiddleware = false
	wasInEnd        = false

	logger = Logger{buffer: bytes.NewBuffer([]byte{})}

	emptyWriter = &ResponseWriter{
		writeBuf: &bytes.Buffer{},
		header:   http.Header{},
	}

	errorDbWriter = &ResponseWriter{
		writeBuf: bytes.NewBuffer([]byte("\"missing parameter: db\"")),
		header: http.Header{
			"Content-Length": []string{"23"},
			"Content-Type":   []string{"application/json"},
		},
		code: 400,
	}

	errorGzipWriter = &ResponseWriter{
		writeBuf: bytes.NewBuffer([]byte("\"unable to decode gzip body\"")),
		header: http.Header{
			"Content-Length": []string{"28"},
			"Content-Type":   []string{"application/json"},
		},
		code: 400,
	}
)

var (
	// w is a ResponseWriter that will be given to the middlewares
	w = &ResponseWriter{
		writeBuf: &bytes.Buffer{},
		header:   http.Header{},
	}
)

func init() {
	wrongGzipBody.buf = &bytes.Buffer{}
	_, err := wrongGzipBody.Write([]byte("WRONG"))
	if err != nil {
		log.Fatal(err)
	}

	gzipBody.buf = &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(gzipBody.buf)
	_, err = gzipWriter.Write([]byte("GOOOOD"))
	if err != nil {
		log.Fatal(err)
	}
	if err = gzipWriter.Close(); err != nil {
		log.Fatal(err)
	}
}

func resetWriter() {
	w.writeBuf = &bytes.Buffer{}
	w.code = 0
	w.header = http.Header{}
}

func WriterTest(t *testing.T, expected *ResponseWriter, actual *ResponseWriter) {
	assert.Equal(t, expected.code, actual.code)
	expectedBuf, _ := ioutil.ReadAll(expected.writeBuf)
	actualBuf, _ := ioutil.ReadAll(actual.writeBuf)
	assert.Equal(t, string(expectedBuf), string(actualBuf))
	assert.Equal(t, expected.header, actual.header)
}

func createHTTP(t *testing.T, cfg config.HTTPConfig, verbose bool) *HTTP {
	tmp, err := NewHTTP(cfg, verbose, config.Filters{})
	if err != nil {
		t.Fatal(err)
	}
	return tmp.(*HTTP)
}

func TestLogMiddleware(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, true)

	h.logger = log.New(logger, "", 0)
	handler := h.logMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	handler(h, w, r, ti)
	buf, _ := ioutil.ReadAll(logger.buffer)
	assert.Equal(t, string(buf), "got request on: influxdb\n")
}

func TestLogMiddlewareNoLog(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)

	h.logger = log.New(logger, "", 0)
	handler := h.logMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	handler(h, w, r, ti)
	buf, _ := ioutil.ReadAll(logger.buffer)
	assert.Equal(t, string(buf), "")
}

func TestQueryMiddleware(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)

	handler := h.queryMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	url, _ := r.URL.Parse("http://influxdb:8086/write?db=mydb")
	r.URL = url
	handler(h, w, r, ti)
	assert.Equal(t, emptyWriter, w)
}

func TestQueryMiddlewareNoDB(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)

	handler := h.queryMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	url, _ := r.URL.Parse("http://influxdb:8086/write")
	r.URL = url
	handler(h, w, r, ti)
	WriterTest(t, errorDbWriter, w)
}

func TestQueryMiddlewareRp(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{DefaultRetentionPolicy: "patate"}, false)

	handler := h.queryMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	url, _ := r.URL.Parse("http://influxdb:8086/write?db=mydb")
	r.URL = url
	handler(h, w, r, ti)
	assert.Equal(t, emptyWriter, w)
	queryParams := r.URL.Query()
	assert.Equal(t, "patate", queryParams.Get("rp"))
}

func TestBodyMiddleWareNoGzip(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)

	handler := h.bodyMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", emptyBody)
	if err != nil {
		t.Fatal(err)
	}
	handler(h, w, r, ti)
	assert.Equal(t, emptyWriter, w)
}

func TestBodyMiddleWareErrorGzip(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)

	handler := h.bodyMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", wrongGzipBody)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Encoding", "gzip")
	handler(h, w, r, ti)
	WriterTest(t, errorGzipWriter, w)
}

func TestBodyMiddleWareGzip(t *testing.T) {
	defer resetWriter()
	h := createHTTP(t, config.HTTPConfig{}, false)
	handler := h.bodyMiddleWare((*HTTP).End)
	r, err := http.NewRequest("", "influxdb", gzipBody)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Encoding", "gzip")
	handler(h, w, r, ti)
	buf, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, "GOOOOD", string(buf))
}

func TestAllMiddlewares(t *testing.T) {
	h := createHTTP(t, config.HTTPConfig{}, false)
	r, err := http.NewRequest("", "influxdb", gzipBody)
	if err != nil {
		t.Fatal(err)
	}

	middlewares = []relayMiddleware{
		(*HTTP).tagMiddleware,
	}
	chain := allMiddlewares(h, (*HTTP).EndTag)
	chain(h, w, r, ti)
	assert.Equal(t, true, wasInMiddleware)
	assert.Equal(t, true, wasInEnd)
}
