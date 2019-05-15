package backend

import (
	"errors"
	"github.com/toni-moreno/influxdb-srelay/config"
	"net/http"
	"time"
)

var (
	mainConfig config.Config
	logDir     string
)

func SetConfig(cfg config.Config) {
	mainConfig = cfg
}

func SetLogdir(ld string) {
	logDir = ld
}

// Default HTTP settings and a few constants
const (
	DefaultHTTPPingResponse = http.StatusNoContent
	DefaultHTTPTimeout      = 10 * time.Second
	DefaultMaxDelayInterval = 10 * time.Second
	DefaultBatchSizeKB      = 512

	KB = 1024
	MB = 1024 * KB
)

// ErrBufferFull error indicates that retry buffer is full
var ErrBufferFull = errors.New("retry buffer full")
