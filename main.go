package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/strike-team/influxdb-relay/config"
	"github.com/strike-team/influxdb-relay/relayservice"
)

const (
	relayVersion = "3.0.2"
)

var (
	usage = func() {
		fmt.Println("Please, see README for more information about InfluxDB Relay...")
		flag.PrintDefaults()
	}

	configFile  = flag.String("config", "", "Configuration file to use")
	verbose     = flag.Bool("v", false, "If set, InfluxDB Relay will log HTTP requests")
	versionFlag = flag.Bool("version", false, "Print current InfluxDB Relay version")
)

func runRelay(cfg config.Config) {
	relay, err := relayservice.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		relay.Stop()
	}()

	log.Println("starting relays...")
	relay.Run()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("influxdb-relay version " + relayVersion)
		return
	}

	// Configuration file is mandatory
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "Missing configuration file")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// And it has to be loaded in order to continue
	cfg, err := config.LoadConfigFile(*configFile)
	if err != nil {
		log.Println("Version: " + relayVersion)
		log.Fatal(err.Error())
	}

	cfg.Verbose = *verbose
	runRelay(cfg)
}
