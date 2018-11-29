package crawler

import (
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/rpc"
	"github.com/ubiq/spectrum-backend/storage"
)

type Config struct {
	Enabled     bool   `json:"enabled"`
	Interval    string `json:"interval"`
	MaxRoutines int    `json:"routines"`
}

type Crawler struct {
	backend *storage.MongoDB
	rpc     *rpc.RPCClient
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

func New(db *storage.MongoDB, rpc *rpc.RPCClient, cfg *Config) *Crawler {
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
	c.ChartBlocktime()
	c.ChartBlocks()
	c.ChartTxns()

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Debugf("Loop: %v, sync: %v", time.Now().UTC(), c.state.syncing)
				c.fetchPrice()
				go c.SyncLoop()
			case <-ticker2.C:
				log.Debugf("Chart Loop: %v", time.Now().UTC())
				go c.ChartTxns()
				go c.ChartBlocks()
			}
		}
	}()

}
