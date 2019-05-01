package config

import (
	"io/ioutil"
	"os"
	//	"regexp"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Influxdb      []*InfluxDBBackend `toml:"influxdb"`
	Influxcluster []*Influxcluster   `toml:"influxcluster"`
	HTTPConfig    []*HTTPConfig      `toml:"http"`
}

func (c *Config) GetInfluxCluster(name string) *Influxcluster {
	for _, k := range c.Influxcluster {
		if k.Name == name {
			return k
		}
	}
	return nil
}

func (c *Config) GetInfluxDBBackend(name string) *InfluxDBBackend {
	for _, k := range c.Influxdb {
		if k.Name == name {
			return k
		}
	}
	return nil
}

type InfluxDBBackend struct {
	Name     string `toml:"name"`
	Location string `toml:"location"`
	Timeout  string `toml:"timeout"`
	// Buffer failed writes up to maximum count (default: 0, retry/buffering disabled)
	BufferSizeMB int `toml:"buffer-size-mb"`

	// Maximum batch size in KB (default: 512)
	MaxBatchKB int `toml:"max-batch-kb"`

	// Maximum delay between retry attempts
	// The format used is the same seen in time.ParseDuration (default: 10s)
	MaxDelayInterval string `toml:"max-delay-interval"`

	// Skip TLS verification in order to use self signed certificate
	// WARNING: It's insecure, use it only for developing and don't use in production
	SkipTLSVerification bool `toml:"skip-tls-verification"`
}

type Influxcluster struct {
	Name                   string   `toml:"name"`
	Members                []string `toml:"members,omitempty"`
	Type                   string   `toml:"type,omitempty"`
	RateLimit              int      `toml:"rate-limit,omitempty"`
	BurstLimit             int      `toml:"burst-limit,omitempty"`
	QueryRouterEndpointAPI []string `toml:"query-router-endpoint-api"`
	DefaultPingResponse    int      `toml:"default-ping-response-code"`
	LogFile                string   `toml:"log-file"`
	LogLevel               string   `toml:"log-level"`
	HealthTimeout          int64    `toml:"health-timeout-ms"`
}

type Filter struct {
	Name   string `toml:"name"`
	Action string `toml:"action"`
	Key    string `toml:"key"`
	Match  string `toml:"match"`
}

type Rule struct {
	Name   string `toml:"name"`
	Action string `toml:"action"`
	Key    string `toml:"key"`
	Match  string `toml:"match"`
	Value  string `toml:"value"`
}

type Route struct {
	Name   string    `toml:"name"`
	Level  string    `toml:"level"`
	Filter []*Filter `toml:"filter"`
	Rule   []*Rule   `toml:"rule"`
}

type Endpoint struct {
	URI          []string `toml:"uri"`
	Type         string   `toml:"type"`
	SourceFormat string   `toml:"source_format"`
	Route        []*Route `toml:"route"`
}

type HTTPConfig struct {
	Name       string      `toml:"name"`
	BindAddr   string      `toml:"bind-addr"`
	LogFile    string      `toml:"log-file"`
	LogLevel   string      `toml:"log-level"`
	RateLimit  int         `toml:"rate-limit,omitempty"`
	BurstLimit int         `toml:"burst-limit,omitempty"`
	Endpoint   []*Endpoint `toml:"endpoint"`
	// Set certificate in order to handle HTTPS requests
	SSLCombinedPem string `toml:"ssl-combined-pem"`
	// Default retention policy to set for forwarded requests
	DefaultRetentionPolicy string `toml:"default-retention-policy"`
}

// LoadConfigFile parses the specified file into a Config object
func LoadConfigFile(filename string) (Config, error) {
	var cfg Config

	f, err := os.Open(filename)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	tomlData, err := ioutil.ReadAll(f)
	if err != nil {
		return cfg, err
	}

	if _, err := toml.Decode(string(tomlData), &cfg); err != nil {
		return cfg, err
	}
	return cfg, err
}
