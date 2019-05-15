package utils

import (
	"encoding/base64"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

var (
	logDir string
)

func SetLogdir(ld string) {
	logDir = ld
}

func ChanToSlice(ch interface{}) interface{} {
	chv := reflect.ValueOf(ch)
	slv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(ch).Elem()), 0, 0)
	for {
		v, ok := chv.Recv()
		if !ok {
			return slv.Interface()
		}
		slv = reflect.Append(slv, v)
	}
}

func GetConsoleLogFormated(logfile string, level string) *zerolog.Logger {
	var i *os.File
	if len(logfile) > 0 {
		filename := logfile
		if !filepath.IsAbs(logfile) {
			filename = filepath.Join(logDir, filename)
		}
		file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		i = file
	} else {
		i = os.Stderr
	}
	f := log.Output(zerolog.ConsoleWriter{Out: i, TimeFormat: "2006-01-02 15:04:05"})
	var logger zerolog.Logger
	switch level {
	case "panic":
		logger = f.Level(zerolog.PanicLevel)
	case "fatal":
		logger = f.Level(zerolog.FatalLevel)
	case "Error", "error":
		logger = f.Level(zerolog.ErrorLevel)
	case "warn", "warning":
		logger = f.Level(zerolog.WarnLevel)
	case "info":
		logger = f.Level(zerolog.InfoLevel)
	case "debug":
		logger = f.Level(zerolog.DebugLevel)
	default:

		logger = f.Level(zerolog.InfoLevel)
	}
	return &logger
}

func GetUserFromRequest(r *http.Request) string {

	username := ""
	found := false
	//check authorization
	auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		found = false
	} else {
		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		username = pair[0]
		found = true
	}

	if !found {
		queryParams := r.URL.Query()
		username = queryParams.Get("u")
	}

	if len(username) > 0 {
		return username
	}
	return "-"

}
