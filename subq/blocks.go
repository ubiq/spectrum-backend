package subq

import (
	"math/big"

	log "github.com/sirupsen/logrus"

	"github.com/ubiq/spectrum-backend/models"
)

func (c *Crawler) SyncLoop() {
	var currentBlock uint64

	indexHead, err := c.backend.LatestSupplyBlock()
	if err != nil {
		log.Errorf("Error getting latest supply block: %v", err)
	}

	chainHead, err := c.rpc.LatestBlockNumber()
	if err != nil {
		log.Errorf("Error getting latest block number: %v", err)
	}

	syncUtility := NewSync()

	if indexHead.Number == 0 {
		// initial sync
		c.state.syncing = true
		currentBlock = 1
	} else {
		if !c.state.syncing {
			// Update state
			c.state.syncing = true
			//c.state.reorg = false
			//syncUtility.setType("resume")

			// log.Warnf("Detected previous unfinished sync, resuming from block %v", indexHead.Number)
			// Purging last blocks from previous sync, in case it was half-synced
			// WARNING: errors from purge can only be not found, we can safely ignore them
			//c.backend.PurgeSupplyFrom(indexHead.Number - 10)
			currentBlock = indexHead.Number + 1
			//c.sbCache.Purge()
			//c.hashCache.Purge()
			//c.backend.Purge(currentBlock+1)
		} else {
			c.state.syncing = true
			currentBlock = indexHead.Number + 1
		}
	}

	syncUtility.setInit(currentBlock)

mainloop:
	for ; currentBlock <= chainHead && !c.state.reorg; currentBlock++ {
		block, err := c.rpc.GetBlockByHeight(currentBlock)
		if err != nil {
			log.Errorf("Error getting block: %v", err)
			c.state.syncing = false
			break mainloop
		}

		if c.state.reorg == true {
			c.state.reorg = false
			c.state.syncing = false
			break mainloop
		}

		syncUtility.add(1)

		go c.Sync(block, syncUtility)

		syncUtility.wait(c.cfg.MaxRoutines)
		syncUtility.swapChannels()
	}

	syncUtility.close(currentBlock)
	c.state.syncing = false
	//if syncUtility.synctype == "back" || syncUtility.synctype == "first" {

	//}

}

func (c *Crawler) Sync(block *models.Block, syncUtility Sync) {
	//log.Warnf("block %v", block.Number)
	syncUtility.recieve()

	var (
		uncles  []*models.Uncle
		pSupply = new(big.Int)
		pHash   string
	)

	// populate uncles
	if len(block.Uncles) > 0 {
		uncles = c.GetUncles(block.Uncles, block.Number)
	}

	// calculate rewards
	blockReward, uncleRewards, minted := AccumulateRewards(block, uncles)

	// get parent block info from cache
	if cached, ok := c.sbCache.Get(block.Number - 1); ok {
		pSupply = cached.(sbCache).Supply
		pHash = cached.(sbCache).Hash
	} else {
		// parent block not cached, fetch from db
		log.Warnf("block %v not found in cache, retrieving from database", block.Number-1)
		lsb, err := c.backend.SupplyBlockByNumber(block.Number - 1)
		if err != nil {
			log.Errorf("Error getting latest supply block: %v", err)
			syncUtility.send(block.Number)
			syncUtility.done()
		} else {
			sprev, _ := new(big.Int).SetString(lsb.Supply, 10)
			pSupply = sprev
			pHash = lsb.Hash
		}
	}

	// check parent hash incase a reorg has occured.
	if pHash != block.ParentHash {
		// a reorg has occured
		log.Warnf("Reorg detected at block %v", block.Number-1)
		// clear cache
		c.sbCache.Purge()
		// remove parent block from db
		c.backend.RemoveSupplyBlock(block.Number - 1)
		c.state.reorg = true

		syncUtility.send(block.Number - 1)
		syncUtility.done()
	} else {
		// add minted to supply
		var supply = new(big.Int)
		supply.Add(pSupply, minted)

		sblock := models.Supply{
			Number:       block.Number,
			Hash:         block.Hash,
			Timestamp:    block.Timestamp,
			BlockReward:  blockReward.String(),
			UncleRewards: uncleRewards.String(),
			Minted:       minted.String(),
			Supply:       supply.String(),
		}

		// write block to db
		err := c.backend.AddSupplyBlock(sblock)
		if err != nil {
			log.Errorf("Error adding block: %v", err)
		}

		// add block to cache for next iteration
		c.sbCache.Add(block.Number, sbCache{Supply: supply, Hash: block.Hash})

		syncUtility.log(block.Number, minted, supply)
		syncUtility.send(block.Number + 1)
		syncUtility.done()
	}
}

func (c *Crawler) GetUncles(uncles []string, height uint64) []*models.Uncle {

	var u []*models.Uncle

	for k, _ := range uncles {
		uncle, err := c.rpc.GetUncleByBlockNumberAndIndex(height, k)
		if err != nil {
			log.Errorf("Error getting uncle: %v", err)
			return u
		}
		u = append(u, uncle)
	}
	return u
}
