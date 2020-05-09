package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	//	"runtime/pprof"
	"syscall"

	"github.com/toni-moreno/influxdb-srelay/backend"
	"github.com/toni-moreno/influxdb-srelay/cluster"
	"github.com/toni-moreno/influxdb-srelay/config"
	"github.com/toni-moreno/influxdb-srelay/relayservice"
	"github.com/toni-moreno/influxdb-srelay/utils"
)

var (
	// Version is the app X.Y.Z version
	Version string
	// Commit is the git commit sha1
	Commit string
	// Branch is the git branch
	Branch string
	// BuildStamp is the build timestamp
	BuildStamp string
)

var (
	usage = func() {
		fmt.Println("Please, see README for more information about InfluxDB Relay...")
		flag.PrintDefaults()
	}

	pidFile     = flag.String("pidfile", "", "path to pid file")
	homeDir     = flag.String("home", "", "path to Home directory(not used yet)")
	dataDir     = flag.String("data", "", "path to Data directory(not used yet)")
	configFile  = flag.String("config", "", "Configuration file to use")
	logDdep     = flag.String("logdir", "", "Default log Directory (deprecated)")
	logD        = flag.String("logs", "", "Default log Directory ")
	versionFlag = flag.Bool("version", false, "Print current InfluxDB Relay version")

	logDir string

	relaysvc  *relayservice.Service
	recsignal os.Signal
	SigMutex  = &sync.RWMutex{}
	RlyMutex  = &sync.RWMutex{}
)

func writePIDFile() {
	if *pidFile == "" {
		return
	}

	// Ensure the required directory structure exists.
	err := os.MkdirAll(filepath.Dir(*pidFile), 0700)
	if err != nil {
		log.Fatal(3, "Failed to verify pid directory", err)
	}

	// Retrieve the PID and write it.
	pid := strconv.Itoa(os.Getpid())
	if err := ioutil.WriteFile(*pidFile, []byte(pid), 0644); err != nil {
		log.Fatal(3, "Failed to write pidfile", err)
	}
}

func StartRelay() error {
	//Load Config File
	var err error
	cfg, err := config.LoadConfigFile(*configFile)
	if err != nil {
		log.Println("Version: ", Version)
		log.Print(err.Error())
		return err
	}

	utils.SetLogdir(logDir)
	utils.SetVersion(Version)
	backend.SetLogdir(logDir)
	backend.SetConfig(cfg)
	cluster.SetLogdir(logDir)
	cluster.SetConfig(cfg)
	RlyMutex.Lock()
	if relaysvc != nil {
		relaysvc.Release()
		relaysvc = nil
	}
	utils.CloseLogFiles()
	utils.ResetLogFiles()
	relaysvc, err = relayservice.New(cfg, logDir)
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
		log.Printf("Error on reload File [%s]: ERR: %s", *configFile, err)
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

	writePIDFile()

	if *versionFlag {
		t, _ := strconv.ParseInt(BuildStamp, 10, 64)
		fmt.Printf("influxdb-srelay v%s (git: %s ) built at [%s]\n", Version, Commit, time.Unix(t, 0).Format("2006-01-02 15:04:05"))
		os.Exit(0)
	}

	// Configuration file is mandatory
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "Missing configuration file")
		flag.PrintDefaults()
		os.Exit(1)
	}

	switch {
	case len(*logD) > 0:
		logDir = *logD
	case len(*logDdep) > 0: //deprecated config
		logDir = *logDdep
	default:
		logDir, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}

	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
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
