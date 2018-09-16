package crawler

import (
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/models"
	"github.com/ubiq/spectrum-backend/rpc"
	"github.com/ubiq/spectrum-backend/storage"
	"github.com/ubiq/spectrum-backend/util"
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
		syncing bool
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
	sync.Mutex
}

var client = &http.Client{Timeout: 60 * time.Second}

func New(db *storage.MongoDB, rpc *rpc.RPCClient, cfg *Config) *Crawler {
	return &Crawler{db, rpc, cfg, struct{ syncing bool }{false}, "0.00000000"}
}

func (c *Crawler) Start() {
	log.Println("Starting block Crawler")

	if c.backend.IsFirstRun() {
		c.backend.Init()
	}

	interval, err := time.ParseDuration(c.cfg.Interval)
	if err != nil {
		log.Fatalf("Crawler: can't parse duration: %v", err)
	}

	timer := time.NewTimer(interval)

	log.Printf("Block refresh interval: %v", interval)

	c.SyncLoop()

	go func() {
		for {
			select {
			case <-timer.C:
				log.Debugf("Loop: %v, sync: %v", time.Now().UTC(), c.state.syncing)
				go c.SyncLoop()
				timer.Reset(interval)
			}
		}
	}()

}

func (c *Crawler) SyncLoop() {
	var wg sync.WaitGroup
	var currentBlock uint64
	var routines int
	var isBacksync bool
	var isFirstsync bool

	// We create two channels, one to recieve values and one to send them
	var c1, c2 chan uint64

	c2 = make(chan uint64, 1)

	start := time.Now()
	indexHead := c.backend.IndexHead()

	if indexHead == 1<<62 {
		isFirstsync = true
		c.state.syncing = true

		startBlock, err := c.rpc.LatestBlockNumber()
		if err != nil {
			log.Errorf("Error getting blockNo: %v", err)
		}
		currentBlock = startBlock

	} else if indexHead != 1<<62 && indexHead != 1 && !c.state.syncing {
		log.Warnf("Detected previous unfinished sync, resuming from block %v", indexHead-1)
		currentBlock = indexHead - 1

		// Update state
		isBacksync = true
		c.state.syncing = true

		// Purging last block from previous sync, in case it was half-synced
		// WARNING: errors from purge can only be not found, we can safely ignore them
		c.backend.Purge(currentBlock)

	} else {
		startBlock, err := c.rpc.LatestBlockNumber()
		if err != nil {
			log.Errorf("Error getting blockNo: %v", err)
		}
		currentBlock = startBlock
	}

	// Send first block into the channel
	c2 <- currentBlock

	// Swap channels
	c1, c2 = c2, make(chan uint64, 1)

	for ; !c.backend.IsPresent(currentBlock); currentBlock-- {
		block, err := c.rpc.GetBlockByHeight(currentBlock)

		if err != nil {
			log.Errorf("Error getting block: %v", err)
		}

		if c.backend.IsPresent(currentBlock) && c.backend.IsForkedBlock(currentBlock, block.Hash) {
			go c.SyncForkedBlock(block, &wg, c1, c2)
		} else {
			go c.Sync(block, &wg, c1, c2)
		}

		wg.Add(1)
		routines++

		if routines == c.cfg.MaxRoutines {
			log.Debugf("Waiting on %v routines", routines)
			wg.Wait()
			routines = 0
		}

		c1, c2 = c2, make(chan uint64, 1)
	}
	// Wait for last goroutine to return before closing the channel
closer:
	for {
		select {
		case close := <-c1:
			if close == currentBlock {
				break closer
			}
		}
	}
	close(c1)
	close(c2)

	if isBacksync || isFirstsync {
		c.state.syncing = false
		log.Errorf("TEST - sync time - %v", time.Since(start))
		os.Exit(1)
	}
}

func (c *Crawler) SyncForkedBlock(block *models.Block, wg *sync.WaitGroup, c1, c2 chan uint64) {

	height := block.Number

	dbblock, err := c.backend.GetBlock(height)
	if err != nil {
		log.Errorf("Error getting forked block: %v", err)
	}

	price := c.getPrice()

	c.backend.AddForkedBlock(dbblock)
	c.backend.UpdateStore(block, dbblock.BlockReward, price, true)
	c.backend.Purge(height)

	log.Warnf("Reorg detected at block: %v", block.Number)
	log.Warnf("HEAD - %v", block.Number)
	log.Warnf("FORK - %v", dbblock.Number)

	c.Sync(block, wg, c1, c2)

}

func (c *Crawler) Sync(block *models.Block, wg *sync.WaitGroup, c1, c2 chan uint64) {

signal:
	for {
		select {
		case sig := <-c1:
			if sig == block.Number {
				log.Debugf("Incoming signal %v %v", sig, block.Number)
				break signal
			}
		}
	}

	close(c1)

	start := time.Now()

	uncleRewards := big.NewInt(0)
	avgGasPrice := big.NewInt(0)
	txFees := big.NewInt(0)
	minted := big.NewInt(0)

	blockReward := util.CaculateBlockReward(block.Number, len(block.Uncles))

	if len(block.Transactions) > 0 {
		avgGasPrice, txFees = c.ProcessTransactions(block.Transactions, block.Timestamp)
	}

	if len(block.Uncles) > 0 {
		uncleRewards = c.ProcessUncles(block.Uncles, block.Number)
	}

	minted.Add(blockReward, uncleRewards)

	block.BlockReward = minted.String()
	block.AvgGasPrice = avgGasPrice.String()
	block.TxFees = txFees.String()
	block.UnclesReward = uncleRewards.String()

	price := c.getPrice()

	err := c.backend.UpdateStore(block, minted.String(), price, false)
	if err != nil {
		log.Errorf("Error updating sysStore: %v", err)
	}

	err = c.backend.AddBlock(block)
	if err != nil {
		log.Errorf("Error adding block: %v", err)
	}

	elapsed := time.Since(start)

	log.Printf("Block (%v): added %v transactions   \t%v uncles\tprice in btc %s\ttook %v", block.Number, len(block.Transactions), len(block.Uncles), price, elapsed)

	log.Debugf("Send signal %v", block.Number-1)
	c2 <- block.Number - 1

	wg.Done()
}

func (c *Crawler) ProcessUncles(uncles []string, height uint64) *big.Int {

	uncleRewards := big.NewInt(0)

	for k, _ := range uncles {

		uncle, err := c.rpc.GetUncleByBlockNumberAndIndex(height, k)
		if err != nil {
			log.Errorf("Error getting uncle: %v", err)
			return big.NewInt(0)
		}

		uncleReward := util.CaculateUncleReward(height, uncle.Number)

		uncleRewards.Add(uncleRewards, uncleReward)

		uncle.BlockNumber = height
		uncle.Reward = uncleReward.String()

		err = c.backend.AddUncle(uncle)

		if err != nil {
			log.Errorf("Error inserting uncle into backend: %v", err)
			return big.NewInt(0)
		}
	}
	return uncleRewards
}

func (c *Crawler) ProcessTransactions(txs []models.RawTransaction, timestamp uint64) (*big.Int, *big.Int) {

	var twg sync.WaitGroup

	start := time.Now()

	data := &data{
		avgGasPrice: big.NewInt(0),
		txFees:      big.NewInt(0),
	}

	for _, v := range txs {
		twg.Add(1)
		go c.processTransaction(v, timestamp, data, &twg)
	}
	twg.Wait()
	log.Debugf("Tansactions took: %v", time.Since(start))
	return data.avgGasPrice.Div(data.avgGasPrice, big.NewInt(int64(len(txs)))), data.txFees
}

func (c *Crawler) processTransaction(rt models.RawTransaction, timestamp uint64, data *data, twg *sync.WaitGroup) {
	v := rt.Convert()

	receipt, err := c.rpc.GetTxReceipt(v.Hash)
	if err != nil {
		log.Errorf("Error getting tx receipt: %v", err)
	}

	data.Lock()
	data.avgGasPrice.Add(data.avgGasPrice, big.NewInt(0).SetUint64(v.Gas))
	data.Unlock()

	gasprice, ok := big.NewInt(0).SetString(v.GasPrice, 10)
	if !ok {
		log.Errorf("Crawler: processTx: couldn't set gasprice (%v): %v", gasprice, ok)
	}

	data.Lock()
	data.txFees.Add(data.txFees, big.NewInt(0).Mul(gasprice, big.NewInt(0).SetUint64(receipt.GasUsed)))
	data.Unlock()

	v.Timestamp = timestamp
	v.GasUsed = receipt.GasUsed
	v.ContractAddress = receipt.ContractAddress
	v.Logs = receipt.Logs

	if v.IsTokenTransfer() {

		tktx := v.GetTokenTransfer()

		tktx.BlockNumber = v.BlockNumber
		tktx.Hash = v.Hash
		tktx.Timestamp = v.Timestamp

		c.backend.AddTokenTransfer(tktx)
	}

	err = c.backend.AddTransaction(v)
	if err != nil {
		log.Errorf("Error inserting tx into backend: %v", err)
	}
	twg.Done()
}

func (c *Crawler) getPrice() string {
	return c.price
}

func (c *Crawler) fetchPrice() {
	var result *apiResponse

	err := util.GetJson(client, "https://bittrex.com/api/v1.1/public/getmarketsummary?market=BTC-UBQ", &result)
	if err != nil {
		log.Print("Could not get price: ", err)
		c.price = "0.00000001"
	}

	x := util.FloatToString(result.Result[0].Last)

	c.price = x
}
