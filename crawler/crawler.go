package crawler

import (
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	mgo "github.com/globalsign/mgo"
	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/models"
)

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

	// iterators
	GetTxnCounts(days int) *mgo.Iter
	GetBlocks(days int) *mgo.Iter
	BlocksIter(blockno uint64) *mgo.Iter
	GetTokenTransfers(contractAddress, address string, after int64) *mgo.Iter

	// setters
	AddTransaction(tx *models.Transaction) error
	AddTokenTransfer(tt *models.TokenTransfer) error
	AddUncle(u *models.Uncle) error
	AddBlock(b *models.Block) error
	AddForkedBlock(b *models.Block) error
	AddLineChart(t *models.LineChart) error
	AddMLChart(t *models.MLineChart) error
}

type Crawler struct {
	backend Database
	rpc     RPCClient
	cfg     *Config
	state   struct {
		syncing    bool
		topsyncing bool
	}
	price string
}

type apiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Result  []struct {
		MarketName     string  `json:"MarketName"`
		High           float64 `json:"High"`
		Low            float64 `json:"Low"`
		Volume         float64 `json:"Volume"`
		Last           float64 `json:"Last"`
		BaseVolume     float64 `json:"BaseVolume"`
		TimeStamp      string  `json:"TimeStamp"`
		Bid            float64 `json:"Bid"`
		Ask            float64 `json:"Ask"`
		OpenBuyOrders  int     `json:"OpenBuyOrders"`
		OpenSellOrders int     `json:"OpenSellOrders"`
		PrevDay        float64 `json:"PrevDay"`
		Created        string  `json:"Created"`
	} `json:"result"`
}

type data struct {
	avgGasPrice, txFees *big.Int
	tokentransfers      int
	sync.Mutex
}

type logObject struct {
	blockNo        uint64
	blocks         int
	txns           int
	tokentransfers int
	uncleNo        int
}

func (l *logObject) add(o *logObject) {
	l.blockNo = o.blockNo
	l.blocks++
	l.txns += o.txns
	l.tokentransfers += o.tokentransfers
	l.uncleNo += o.uncleNo
}

func (l *logObject) clear() {
	l.txns = 0
	l.tokentransfers = 0
	l.uncleNo = 0
	l.blocks = 0
	l.blockNo = 0
}

var client = &http.Client{Timeout: 60 * time.Second}

func New(db Database, rpc RPCClient, cfg *Config) *Crawler {
	return &Crawler{db, rpc, cfg, struct{ syncing, topsyncing bool }{false, false}, "0.00000000"}
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
	ticker2 := time.NewTicker(10 * time.Minute)

	log.Printf("Block refresh interval: %v", interval)

	go c.SyncLoop()
	c.StoreUbqSupply()
	c.StoreQwarkSupply()
	c.ChartBlocktime()
	c.ChartMinedBlocks()
	c.ChartBlocks()
	c.ChartTxns()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Debugf("Loop: %v, sync: %v", time.Now().UTC(), c.state.syncing)
				c.fetchPrice()
				c.StoreUbqSupply()
				go c.SyncLoop()
			case <-ticker2.C:
				log.Debugf("Chart Loop: %v", time.Now().UTC())
				go c.StoreQwarkSupply()
				go c.ChartBlocktime()
				go c.ChartMinedBlocks()
				go c.ChartTxns()
				go c.ChartBlocks()
			}
		}
	}()

}
