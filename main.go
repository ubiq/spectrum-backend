package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/api"
	"github.com/ubiq/spectrum-backend/config"
	"github.com/ubiq/spectrum-backend/crawler"
	"github.com/ubiq/spectrum-backend/rpc"
	"github.com/ubiq/spectrum-backend/storage"
)

var cfg config.Config

func init() {

	v, _ := strconv.ParseBool(os.Getenv("DEBUG"))
	if v {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.Stamp})
		log.SetLevel(log.InfoLevel)
	}
}

func readConfig(cfg *config.Config) {

	if len(os.Args) == 1 {
		log.Fatalln("Please specify config")
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

func startCrawler(mongo *storage.MongoDB, rpc *rpc.RPCClient, cfg *crawler.Config) {
	c := crawler.New(mongo, rpc, cfg)
	c.Start()
}

func startApi(mongo *storage.MongoDB, cfg *api.Config) {
	a := api.New(mongo, cfg)
	a.Start()
}

func main() {
	readConfig(&cfg)

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

	// TODO: Should be safe to run both concurrently, but for now one or the other

	if cfg.Crawler.Enabled && !cfg.Api.Enabled {
		go startCrawler(mongo, rpc, &cfg.Crawler)
	} else if cfg.Api.Enabled && !cfg.Crawler.Enabled {
		go startApi(mongo, &cfg.Api)
	} else {
		log.Fatalf("Cannot run both api and crawler services at the same time")
	}

	quit := make(chan bool)
	<-quit
}
