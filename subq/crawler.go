package subq

import (
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	mgo "github.com/globalsign/mgo"
	log "github.com/sirupsen/logrus"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ubiq/spectrum-backend/models"
)

const (
	sbCacheLimit = 10
)

type sbCache struct {
	Supply *big.Int `json:"supply"`
	Hash   string   `json:"hash"`
}

type Config struct {
	Enabled     bool   `json:"enabled"`
	Interval    string `json:"interval"`
	MaxRoutines int    `json:"routines"`
}

type RPCClient interface {
	GetLatestBlock() (*models.Block, error)
	GetBlockByHeight(height uint64) (*models.Block, error)
	GetBlockByHash(hash string) (*models.Block, error)
	GetUncleByBlockNumberAndIndex(height uint64, index int) (*models.Uncle, error)
	LatestBlockNumber() (uint64, error)
	GetTxReceipt(hash string) (*models.TxReceipt, error)
	Ping() error
}

type Database interface {
	// Init
	Init()

	// storage
	IsFirstRun() bool
	IsPresent(height uint64) bool
	IsInDB(height uint64, hash string) (bool, bool)
	IndexHead() [1]uint64
	UpdateStore(latestBlock *models.Block, synctype string) error
	SupplyObject(symbol string) (models.Store, error)
	UpdateSupply(ticker string, new *models.Store) error
	GetBlock(height uint64) (*models.Block, error)
	Purge(height uint64)
	Ping() error
	LatestSupplyBlock() (models.Supply, error)
	SupplyBlockByNumber(height uint64) (*models.Supply, error)
	RemoveSupplyBlock(height uint64) error
	// iterators
	GetTxnCounts(days int) *mgo.Iter
	GetBlocks(days int) *mgo.Iter
	BlocksIter(blockno uint64) *mgo.Iter
	GetTokenTransfers(contractAddress, address string, after int64) *mgo.Iter

	// setters
	AddSupplyBlock(b models.Supply) error
}

type Crawler struct {
	backend Database
	rpc     RPCClient
	cfg     *Config
	state   struct {
		syncing bool
		reorg   bool
	}
	sbCache   *lru.Cache // Cache for the most recent supply blocks
	hashCache *lru.Cache // Cache for the most recent block hashes
}

type logObject struct {
	blockNo uint64
	blocks  int
	minted  *big.Int
	supply  *big.Int
}

func (l *logObject) add(o *logObject) {
	l.blockNo = o.blockNo
	l.blocks++
	l.minted.Add(l.minted, o.minted)
	l.supply = o.supply
}

func (l *logObject) clear() {
	l.blockNo = 0
	l.blocks = 0
	l.minted = new(big.Int)
	l.supply = new(big.Int)
}

var client = &http.Client{Timeout: 60 * time.Second}

func New(db Database, rpc RPCClient, cfg *Config) *Crawler {
	sbc, _ := lru.New(sbCacheLimit)
	hc, _ := lru.New(sbCacheLimit)
	return &Crawler{db, rpc, cfg, struct{ syncing, reorg bool }{false, false}, sbc, hc}
}

func (c *Crawler) Start() {
	log.Println("Starting block Crawler")

	err := c.rpc.Ping()

	if err != nil {
		if err == err.(*url.Error) {
			log.Errorf("Gubiq node offline")
			os.Exit(1)
		} else {
			log.Errorf("Error pinging rpc node: %#v", err)
		}
	}

	if c.backend.IsFirstRun() {
		c.backend.Init()
	}

	interval, err := time.ParseDuration(c.cfg.Interval)
	if err != nil {
		log.Fatalf("Crawler: can't parse duration: %v", err)
	}

	ticker := time.NewTicker(interval)

	log.Printf("Block refresh interval: %v", interval)

	go c.SyncLoop()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Debugf("Loop: %v, sync: %v", time.Now().UTC(), c.state.syncing)
				if !c.state.syncing {
					go c.SyncLoop()
				}
			}
		}
	}()

}
