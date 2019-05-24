package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/config"
	"github.com/ubiq/spectrum-backend/subq"
	"github.com/ubiq/spectrum-backend/params"
	"github.com/ubiq/spectrum-backend/rpc"
	"github.com/ubiq/spectrum-backend/storage"
)

var cfg config.Config

func init() {

	v, _ := strconv.ParseBool(os.Getenv("DEBUG"))
	if v {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})
		log.SetLevel(log.DebugLevel)
		log.SetReportCaller(true)
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.Stamp})
		log.SetLevel(log.InfoLevel)
	}
}

func readConfig(cfg *config.Config) {

	if len(os.Args) == 1 {
		log.Fatalln("Invalid arguments")
	}

	conf := os.Args[1]
	conf, _ = filepath.Abs(conf)

	log.Printf("Loading config: %v", conf)

	configFile, err := os.Open(conf)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		log.Fatal("Config error: ", err.Error())
	}
}

func startCrawler(mongo *storage.MongoDB, rpc *rpc.RPCClient, cfg *subq.Config) {
	c := subq.New(mongo, rpc, cfg)
	c.Start()
}

func main() {
	log.Info("subq ", params.VersionWithMeta, " (", params.VersionWithCommit, ")")

	readConfig(&cfg)

	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		log.Printf("Running with %v threads", cfg.Threads)
	} else {
		runtime.GOMAXPROCS(1)
		log.Println("Running with 1 thread")
	}

	mongo, err := storage.NewConnection(&cfg.Mongo)

	if err != nil {
		log.Fatalf("Can't establish connection to mongo: %v", err)
	} else {
		log.Printf("Successfully connected to mongo at %v", cfg.Mongo.Address)
	}

	err = mongo.Ping()

	if err != nil {
		log.Printf("Can't establish connection to mongo: %v", err)
	} else {
		log.Println("PING")
	}

	rpc := rpc.NewRPCClient(&cfg.Rpc)

	if cfg.Subq.Enabled {
		go startCrawler(mongo, rpc, &cfg.Subq)
	} else {
		log.Fatalf("Subq not enabled in config.")
	}

	quit := make(chan bool)
	<-quit
}
