package crawler

import (
	"math/big"
	"net/http"
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

var client = &http.Client{Timeout: 60 * time.Second}

func New(db *storage.MongoDB, rpc *rpc.RPCClient, cfg *Config) *Crawler {
	return &Crawler{db, rpc, cfg}
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
	go func() {
		for {
			select {
			case <-timer.C:
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

	startBlock, err := c.rpc.LatestBlockNumber()

	if err != nil {
		log.Errorf("Error getting blockNo: %v", err)
	}

	for currentBlock = startBlock; !c.backend.IsPresent(currentBlock); currentBlock-- {

		block, err := c.rpc.GetBlockByHeight(currentBlock)

		if err != nil {
			log.Errorf("Error getting block: %v", err)
		}

		// TODO: try to test this

		if c.backend.IsPresent(currentBlock) && c.backend.IsForkedBlock(currentBlock, block.Hash) {
			go c.SyncForkedBlock(block, &wg)
		} else {
			go c.Sync(block, &wg)
		}

		wg.Add(1)
		routines++

		if routines == c.cfg.MaxRoutines {
			wg.Wait()
			routines = 0
		}
	}
}

func (c *Crawler) SyncForkedBlock(block *models.Block, wg *sync.WaitGroup) {
	defer wg.Done()

	height := block.Number

	dbblock, err := c.backend.GetBlock(height)

	if err != nil {
		log.Errorf("Error getting forked block: %v", err)
	}

	price := c.getPrice()

	c.backend.AddForkedBlock(dbblock)

	c.backend.UpdateStore(dbblock.BlockReward, block, price, true)

	c.backend.ReorgPurge(height)

	log.Warnf("Reorg detected at block: %v", block.Number)
	log.Warnf("HEAD - %v", block.Number)
	log.Warnf("FORK - %v", dbblock.Number)

	c.Sync(block, wg)

}

func (c *Crawler) Sync(block *models.Block, wg *sync.WaitGroup) {
	defer wg.Done()

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

	err := c.backend.UpdateStore(minted.String(), block, price, false)

	if err != nil {
		log.Errorf("Error updating sysStore: %v", err)
	}

	err = c.backend.AddBlock(block)

	if err != nil {
		log.Errorf("Error adding block: %v", err)
	}

	log.Printf("Block (%v): added %v transactions, %v uncles; price in btc %s", block.Number, len(block.Transactions), len(block.Uncles), price)

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

	avgGasPrice := big.NewInt(0)
	txFees := big.NewInt(0)

	for _, v := range txs {

		v := v.Convert()

		receipt, err := c.rpc.GetTxReceipt(v.Hash)

		if err != nil {
			log.Errorf("Error getting tx receipt: %v", err)
		}

		avgGasPrice.Add(avgGasPrice, big.NewInt(0).SetUint64(v.Gas))

		gasprice, ok := big.NewInt(0).SetString(v.GasPrice, 10)

		if !ok {
			log.Errorf("Crawler: processTx: couldn't set gasprice (%v): %v", gasprice, ok)
		}

		txFees.Add(txFees, big.NewInt(0).Mul(gasprice, big.NewInt(0).SetUint64(receipt.GasUsed)))

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

	}
	return avgGasPrice.Div(avgGasPrice, big.NewInt(int64(len(txs)))), txFees
}

func (c *Crawler) getPrice() string {
	var result *apiResponse

	err := util.GetJson(client, "https://bittrex.com/api/v1.1/public/getmarketsummary?market=BTC-UBQ", &result)

	if err != nil {
		log.Print("Could not get price: ", err)
		return "0.00000001"
	}

	x := util.FloatToString(result.Result[0].Last)

	return x
}
