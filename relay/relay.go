package relay

import (
	"fmt"
	"github.com/toni-moreno/influxdb-srelay/config"
)

// Relay is an HTTP or UDP endpoint
type Relay interface {
	Name() string
	Run() error
	Stop() error
}

var (
	mainConfig config.Config
	clusters   map[string]*Cluster
)

func SetConfig(cfg config.Config) {
	mainConfig = cfg
}

func InitClusters() error {

	clusters = make(map[string]*Cluster)

	for _, cfg := range mainConfig.Influxcluster {

		c, err := NewCluster(cfg)
		if err != nil {
			return err
		}
		if clusters[cfg.Name] != nil {
			return fmt.Errorf("duplicate cluster: %q", cfg.Name)
		}
		clusters[cfg.Name] = c
	}
	return nil
}
