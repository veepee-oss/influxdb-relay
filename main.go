package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	//	"runtime/pprof"
	"syscall"

	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/cluster"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayservice"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

const (
	relayVersion = "0.3.0"
)

var (
	usage = func() {
		fmt.Println("Please, see README for more information about InfluxDB Relay...")
		flag.PrintDefaults()
	}

	configFile  = flag.String("config", "", "Configuration file to use")
	logDir      = flag.String("logdir", "", "Default log Directory")
	verbose     = flag.Bool("v", false, "If set, InfluxDB Relay will log HTTP requests")
	versionFlag = flag.Bool("version", false, "Print current InfluxDB Relay version")

	relay     *relayservice.Service
	recsignal os.Signal
)

func StartRelay() error {
	//Load Config File
	var err error
	cfg, err := config.LoadConfigFile(*configFile)
	if err != nil {
		log.Println("Version: " + relayVersion)
		log.Fatal(err.Error())
		return err
	}
	utils.SetLogdir(*logDir)
	utils.SetVersion(relayVersion)
	backend.SetLogdir(*logDir)
	backend.SetConfig(cfg)
	cluster.SetLogdir(*logDir)
	cluster.SetConfig(cfg)

	relay, err = relayservice.New(cfg, *logDir)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Println("starting relays...")
	return relay.Run()
}

func ReloadRelay() {
	//validate conf
	cfg, err := config.LoadConfigFile(*configFile)
	if err != nil {
		log.Printf("Error on reload File [%s]: ERR: %s", configFile, err)
		return
	}
	log.Printf("Reloading Config File %#+v", cfg)
	//config ok
	relay.Stop()
}

func main() {

	var err error

	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("influxdb-srelay version " + relayVersion)
		return
	}

	// Configuration file is mandatory
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "Missing configuration file")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*logDir) == 0 {
		*logDir, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		//check if exist and
		if _, err := os.Stat(*logDir); os.IsNotExist(err) {
			os.Mkdir(*logDir, 0755)
		}
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGINT)
	go func() {
		for {

			select {
			case sig := <-c:
				recsignal = sig
				switch sig {
				case syscall.SIGTERM, syscall.SIGQUIT: // 15,3
					log.Printf("Received TERM signal")
					relay.Stop()
					log.Printf("Exiting for requested user SIGTERM")
					os.Exit(1)
				case syscall.SIGHUP: // 1
					log.Printf("Received HUP signal")
					ReloadRelay()
					log.Printf("Relay Reloaded OK")
					/*	case syscall.SIGINT: //2
						log.Printf("Received INT signal, print stack trace")

						pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
						pprof.Lookup("block").WriteTo(os.Stdout, 1)*/
				default:
					log.Printf("Received Unknown signal %q", sig)
				}

			}

		}

	}()

	for {
		err := StartRelay()
		if err != nil {
			log.Fatal("ERROR on start Relay : %s", err)
			os.Exit(1)
		}
		switch recsignal {
		case syscall.SIGTERM, syscall.SIGQUIT:
			os.Exit(0)
		}
	}

}
