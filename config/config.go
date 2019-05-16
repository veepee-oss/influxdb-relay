package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

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

func (ib *InfluxDBBackend) ValidateCfg(cfg *Config) error {
	return nil
}

type ClusterType string

const (
	ClType_HA     ClusterType = "HA"
	ClType_Single ClusterType = "Single"
	ClType_LB     ClusterType = "LB"
)

type Influxcluster struct {
	Name                   string      `toml:"name"`
	Members                []string    `toml:"members,omitempty"`
	Type                   ClusterType `toml:"type,omitempty"`
	RateLimit              int         `toml:"rate-limit,omitempty"`
	BurstLimit             int         `toml:"burst-limit,omitempty"`
	QueryRouterEndpointAPI []string    `toml:"query-router-endpoint-api"`
	DefaultPingResponse    int         `toml:"default-ping-response-code"`
	LogFile                string      `toml:"log-file"`
	LogLevel               string      `toml:"log-level"`
	HealthTimeout          int64       `toml:"health-timeout-ms"`
}

func (ic *Influxcluster) ValidateCfg(cfg *Config) error {
	//Validate type
	switch ic.Type {
	case ClType_HA:
		if len(ic.Members) < 2 {
			return fmt.Errorf("Error on Cluster %s : on HA clusters members shoud be >= 2", ic.Name)
		}
	case ClType_Single:
		if len(ic.Members) != 1 {
			return fmt.Errorf("Error on Cluster %s : on Single clusters members shoud be = 1", ic.Name)
		}
	case ClType_LB:
		return fmt.Errorf("SORRY LB Clusters not yet supported", ic.Name)
	default:
		return fmt.Errorf("Error in parse Cluster %s: Unteknown Type %s", ic.Name, ic.Type)
	}
	//validate if exist to_cluster
	for _, m := range ic.Members {
		bcfg := cfg.GetInfluxDBBackend(m)
		if bcfg == nil {
			return fmt.Errorf("Error on Cluster %s : invalid member %s ", ic.Name, m)
		}
	}
	return nil
}

type RuleKey string

const (
	//HTTP Header Based
	RKey_authorization  RuleKey = "authorization"
	RKey_remote_address RuleKey = "remote-address"
	RKey_referer        RuleKey = "referer"
	RKey_user_agent     RuleKey = "user-agent"
	RKey_username       RuleKey = "username"
	//HTTP Parameter Based
	RKey_db          RuleKey = "db"
	RKey_q           RuleKey = "q"
	RKey_epoch       RuleKey = "epoch"
	RKey_chunked     RuleKey = "chunked"
	RKey_chunksize   RuleKey = "chunksize"
	RKey_pretty      RuleKey = "pretty"
	RKey_u           RuleKey = "u"
	RKey_p           RuleKey = "p"
	RKey_rp          RuleKey = "rp"
	RKey_precision   RuleKey = "precision"
	RKey_consistency RuleKey = "consistency"

	//Data Based
	RKey_measurement RuleKey = "measurement"
	RKey_field       RuleKey = "field"
	RKey_tag         RuleKey = "tag"
)

func ValidateKey(name string, key RuleKey) error {
	//Validate RuleKey

	switch key {
	//HTTP Parameter Based
	case RKey_authorization:
	case RKey_remote_address:
	case RKey_referer:
	case RKey_user_agent:
	case RKey_username:
		//HTTP Parameter Based
	case RKey_db:
	case RKey_q:
	case RKey_epoch:
	case RKey_chunked:
	case RKey_chunksize:
	case RKey_pretty:
	case RKey_u:
	case RKey_p:
	case RKey_rp:
	case RKey_precision:
	case RKey_consistency:
	case RKey_measurement:
	case RKey_field:
	case RKey_tag:
	default:
		return fmt.Errorf("Error in parse Rule %s: Invalid Key %s", name, key)
	}
	return nil
}

type Filter struct {
	Name  string  `toml:"name"`
	Key   RuleKey `toml:"key"`
	Match string  `toml:"match"`
}

func (f *Filter) ValidateCfg(cfg *Config) error {
	// Validate Key

	err := ValidateKey(f.Name, f.Key)
	if err != nil {
		return err
	}
	//check if match is a valid Regexp
	_, err = regexp.Compile(f.Match)
	if err != nil {
		return fmt.Errorf("Error on filter %s :invalid match regexp  %s err : %s", f.Name, f.Match, err)
	}
	return nil
}

type RuleAction string

const (
	RuleAct_Route        RuleAction = "route"
	RuleAct_RouteDBfData RuleAction = "route_db_from_data"
	RuleAct_RenameHTTP   RuleAction = "rename_http"
	RuleAct_RenameData   RuleAction = "rename_data"
	RuleAct_DropData     RuleAction = "drop_data"
	RuleAct_Break        RuleAction = "break"
)

type Rule struct {
	Name           string     `toml:"name"`
	Action         RuleAction `toml:"action"`
	Key            RuleKey    `toml:"key"` //	"regexp"
	KeyAux         string     `toml:"key_aux"`
	Match          string     `toml:"match"`
	Value          string     `toml:"value"`
	ValueOnUnMatch string     `toml:"value_on_unmatch`
	ToCluster      string     `toml:"to_cluster"`
}

func (r *Rule) ValidateCfg(cfg *Config) error {
	//Validate Action
	switch r.Action {
	case RuleAct_Route:
		fallthrough
	case RuleAct_RouteDBfData:
		if len(r.ToCluster) == 0 {
			return fmt.Errorf("Error in parse Rule %s: Unknown Param to_cluster (should be set)", r.Name)
		}
	case RuleAct_RenameHTTP:
	case RuleAct_RenameData:
	case RuleAct_DropData:
	case RuleAct_Break:
	default:
		return fmt.Errorf("Error in parse Rule %s: Unknown Action %s", r.Name, r.Action)
	}

	err := ValidateKey(r.Name, r.Key)
	if err != nil {
		return err
	}

	//Validate regex
	//check if match is a valid Regexp
	_, err = regexp.Compile(r.Match)
	if err != nil {
		return fmt.Errorf("Error on Rule %s :invalid match regexp  %s err : %s", r.Name, r.Match, err)
	}
	//validate if exist to_cluster
	if len(r.ToCluster) > 0 {

		c := cfg.GetInfluxCluster(r.ToCluster)
		if c == nil && r.ToCluster != "__sinc__" {
			return fmt.Errorf("Error on Rule %s :invalid SendTo %s err : Any Cluster exist with this ID", r.Name, r.ToCluster)
		}
	}
	return nil

}

type RouteLevel string

const (
	RouteLvl_http RouteLevel = "http"
	RouteLvl_data RouteLevel = "data"
)

type Route struct {
	Name       string     `toml:"name"`
	Level      RouteLevel `toml:"level"`
	Filter     []*Filter  `toml:"filter"`
	Rule       []*Rule    `toml:"rule"`
	LogInherit bool       `toml:"log-inherit"`
	LogFile    string     `toml:"log-file"`
	LogLevel   string     `toml:"log-level"`
}

func (r *Route) ValidateCfg(cfg *Config) error {
	//ValidateLevel
	switch r.Level {
	case RouteLvl_data:
	case RouteLvl_http:
	default:
		return fmt.Errorf("Error in parse Route %s: Invalid Level %s", r.Name, r.Level)
	}
	//Validate Filters.
	for _, f := range r.Filter {
		err := f.ValidateCfg(cfg)
		if err != nil {
			return fmt.Errorf("Error in  Route %s  Filter\n \t %s", r.Name, err)
		}
	}
	//Validate Rules
	for _, r := range r.Rule {
		err := r.ValidateCfg(cfg)
		if err != nil {
			return fmt.Errorf("Error in  Route %s  Rule\n \t %s", r.Name, err)
		}
	}
	return nil
}

type EndPType string

const (
	EndPType_RD EndPType = "RD"
	EndPType_WR EndPType = "WR"
)

type EndPSFormat string

const (
	EndPSFormat_IQL    EndPSFormat = "IQL"        //Influx Query Language
	EndPSFormat_ILP    EndPSFormat = "ILP"        //Influx Line Protocol
	EndPSFormat_promwr EndPSFormat = "prom-write" // Prometheus Write
)

type Endpoint struct {
	URI          []string    `toml:"uri"`
	Type         EndPType    `toml:"type"`
	SourceFormat EndPSFormat `toml:"source_format"`
	Route        []*Route    `toml:"route"`
}

func (e *Endpoint) ValidateCfg(cfg *Config) error {
	// Check if type ok
	switch e.Type {
	case EndPType_RD:
	case EndPType_WR:
	default:
		return fmt.Errorf("Error in parse Endpoint %s: Invalid Type %s", e.URI, e.Type)
	}

	switch e.SourceFormat {
	case EndPSFormat_IQL:
	case EndPSFormat_ILP:
	case EndPSFormat_promwr:
	default:
		return fmt.Errorf("Error in parse Endpoint %s: Invalid SourceFormat %s", e.URI, e.SourceFormat)
	}

	for _, r := range e.Route {
		err := r.ValidateCfg(cfg)
		if err != nil {
			return fmt.Errorf("Error in parse Endpoint %s: Invalid Route Err: %s ", e.URI, err)
		}
	}

	return nil
}

type HTTPConfig struct {
	Name       string      `toml:"name"`
	BindAddr   string      `toml:"bind-addr"`
	LogFile    string      `toml:"log-file"`
	LogLevel   string      `toml:"log-level"`
	AccessLog  string      `toml:"access-log"`
	RateLimit  int         `toml:"rate-limit,omitempty"`
	BurstLimit int         `toml:"burst-limit,omitempty"`
	Endpoint   []*Endpoint `toml:"endpoint"`
	// Set certificate in order to handle HTTPS requests
	SSLCombinedPem string `toml:"ssl-combined-pem"`
	// Default retention policy to set for forwarded requests
	DefaultRetentionPolicy string `toml:"default-retention-policy"`
}

func (h *HTTPConfig) ValidateCfg(cfg *Config) error {
	for _, e := range h.Endpoint {
		err := e.ValidateCfg(cfg)
		if err != nil {
			return fmt.Errorf("Error in Endpoint %s : Err : %s ", e.URI, err)
		}
	}
	return nil
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
	//Validate Cluster Config
	for _, c := range cfg.Influxcluster {
		err := c.ValidateCfg(&cfg)
		if err != nil {
			return cfg, err
		}
	}
	//Validate Backend Config
	for _, b := range cfg.Influxdb {
		err := b.ValidateCfg(&cfg)
		if err != nil {
			return cfg, err
		}
	}

	//Validate Backend Config
	for _, h := range cfg.HTTPConfig {
		err := h.ValidateCfg(&cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, err
}
