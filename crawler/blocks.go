package crawler

import (
	"math/big"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/models"
	"github.com/ubiq/spectrum-backend/util"
)

func (c *Crawler) SyncLoop() {
	var wg sync.WaitGroup
	var currentBlock uint64
	var routines int
	var isBacksync bool
	var isFirstsync bool

	// We create two channels, one to recieve values and one to send them
	var c1, c2 chan uint64

	c2 = make(chan uint64, 1)

	indexHead := c.backend.IndexHead()

	if indexHead[0] == 1<<62 {
		isFirstsync = true
		c.state.syncing = true

		startBlock, err := c.rpc.LatestBlockNumber()
		if err != nil {
			log.Errorf("Error getting blockNo: %v", err)
		}
		currentBlock = startBlock
	} else if indexHead[0] == 0 {

		startBlock, err := c.rpc.LatestBlockNumber()
		if err != nil {
			log.Errorf("Error getting blockNo: %v", err)
		}
		currentBlock = startBlock

	} else {

		if !c.state.syncing {
			log.Warnf("Detected previous unfinished sync, resuming from block %v", indexHead[0]-1)
			currentBlock = indexHead[0] - 1

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
	}

	// Send first block into the channel
	c2 <- currentBlock

	// Swap channels
	c1, c2 = c2, make(chan uint64, 1)

mainloop:
	for ; !c.backend.IsPresent(currentBlock); currentBlock-- {
		block, err := c.rpc.GetBlockByHeight(currentBlock)

		if err != nil {
			log.Errorf("Error getting block: %v", err)
		}

		wg.Add(1)

		// FIXME: this reorg code will never be reached as isPresent won't trigger the loop

		if c.backend.IsPresent(currentBlock) && c.backend.IsForkedBlock(currentBlock, block.Hash) {
			go c.SyncForkedBlock(block, &wg, c1, c2)
		} else if !c.backend.IsPresent(currentBlock) {
			go c.Sync(block, &wg, c1, c2)
		} else {
			break mainloop
		}

		routines++

		if routines == c.cfg.MaxRoutines {
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

	<-c1
	close(c1)

	var tokentransfers int
	start := time.Now()

	uncleRewards := big.NewInt(0)
	avgGasPrice := big.NewInt(0)
	txFees := big.NewInt(0)
	minted := big.NewInt(0)

	blockReward := util.CaculateBlockReward(block.Number, len(block.Uncles))

	if len(block.Transactions) > 0 {
		avgGasPrice, txFees, tokentransfers = c.ProcessTransactions(block.Transactions, block.Timestamp)
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

	log.Printf("Added block %v   \twith     \t%v transactions   \t%v tokentransfers   \t%v uncles\ttook %v", block.Number, block.Txs, tokentransfers, block.UncleNo, time.Since(start))

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

func (c *Crawler) ProcessTransactions(txs []models.RawTransaction, timestamp uint64) (*big.Int, *big.Int, int) {

	var twg sync.WaitGroup

	data := &data{
		avgGasPrice:    big.NewInt(0),
		txFees:         big.NewInt(0),
		tokentransfers: 0,
	}

	twg.Add(len(txs))

	for _, v := range txs {
		go c.processTransaction(v, timestamp, data, &twg)
	}
	twg.Wait()
	return data.avgGasPrice.Div(data.avgGasPrice, big.NewInt(int64(len(txs)))), data.txFees, data.tokentransfers
}

func (c *Crawler) processTransaction(rt models.RawTransaction, timestamp uint64, data *data, twg *sync.WaitGroup) {

	v := rt.Convert()

	// Create a channel

	ch := make(chan struct{}, 1)

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
		// Here we fork to a secondary process to insert the token transfer.
		// We use the channel to block the function util the token transfer is inserted.
		data.Lock()
		data.tokentransfers++
		data.Unlock()
		go c.processTokenTransfer(v, ch)
	}

	err = c.backend.AddTransaction(v)
	if err != nil {
		log.Errorf("Error inserting tx into backend: %#v", err)

	}

	if v.IsTokenTransfer() {
		<-ch
	}
	twg.Done()
}

func (c *Crawler) processTokenTransfer(v *models.Transaction, ch chan struct{}) {

	tktx := v.GetTokenTransfer()

	tktx.BlockNumber = v.BlockNumber
	tktx.Hash = v.Hash
	tktx.Timestamp = v.Timestamp

	err := c.backend.AddTokenTransfer(tktx)
	if err != nil {
		log.Errorf("Error processing token transfer into backend: %v", err)
	}

	ch <- struct{}{}

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
		return
	}

	x := util.FloatToString(result.Result[0].Last)

	c.price = x
}