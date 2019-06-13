package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	//	"runtime/pprof"
	"syscall"

	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/cluster"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayservice"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

const (
	relayVersion = "0.5.0"
)

var (
	usage = func() {
		fmt.Println("Please, see README for more information about InfluxDB Relay...")
		flag.PrintDefaults()
	}

	configFile  = flag.String("config", "", "Configuration file to use")
	logDir      = flag.String("logdir", "", "Default log Directory")
	versionFlag = flag.Bool("version", false, "Print current InfluxDB Relay version")

	relaysvc  *relayservice.Service
	recsignal os.Signal
	SigMutex  = &sync.RWMutex{}
	RlyMutex  = &sync.RWMutex{}
)

func StartRelay() error {
	//Load Config File
	var err error
	cfg, err := config.LoadConfigFile(*configFile)
	if err != nil {
		log.Println("Version: " + relayVersion)
		log.Print(err.Error())
		return err
	}

	utils.SetLogdir(*logDir)
	utils.SetVersion(relayVersion)
	backend.SetLogdir(*logDir)
	backend.SetConfig(cfg)
	cluster.SetLogdir(*logDir)
	cluster.SetConfig(cfg)
	RlyMutex.Lock()
	if relaysvc != nil {
		relaysvc.Release()
		relaysvc = nil
	}
	utils.CloseLogFiles()
	utils.ResetLogFiles()
	relaysvc, err = relayservice.New(cfg, *logDir)
	if err != nil {
		log.Print(err)
		RlyMutex.Unlock()
		return err
	}
	r := relaysvc
	RlyMutex.Unlock()
	log.Println("starting relays...")
	return r.Run()
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
	RlyMutex.Lock()
	relaysvc.Stop()
	RlyMutex.Unlock()

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
				SigMutex.RLock()
				recsignal = sig
				SigMutex.RUnlock()
				switch sig {
				case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT: // 15,3
					log.Printf("Received TERM signal")
					RlyMutex.RLock()
					relaysvc.Stop()
					RlyMutex.RUnlock()
					log.Printf("Exiting for requested user SIGTERM")
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
			log.Printf("ERROR on start Relay : %s", err)
			os.Exit(1)
		}
		SigMutex.Lock()
		sig := recsignal
		SigMutex.Unlock()
		switch sig {
		case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
			os.Exit(0)
		}
	}

}
