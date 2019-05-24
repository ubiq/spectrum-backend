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
		syncUtility.setType("first")
		c.state.syncing = true
		var startBlock uint64 = 1
		currentBlock = startBlock
	} else {
		if !c.state.syncing && !c.state.topsyncing {
			log.Warnf("Detected previous unfinished sync, resuming from block %v", indexHead.Number+1)
			currentBlock = indexHead.Number+1

			// Update state
			syncUtility.setType("resume")
			c.state.syncing = true

			// Purging last block from previous sync, in case it was half-synced
			// WARNING: errors from purge can only be not found, we can safely ignore them
			//c.backend.Purge(currentBlock) TODO
      //c.backend.Purge(currentBlock+1)
		} else {
			currentBlock = indexHead.Number + 1
		}
	}

	syncUtility.setInit(currentBlock)

mainloop:
	for ; currentBlock < chainHead; currentBlock++ {
		block, err := c.rpc.GetBlockByHeight(currentBlock)

		if err != nil {
			log.Errorf("Error getting block: %v", err)
      break mainloop
		}

		syncUtility.add(1)

    _, e := c.backend.SupplyBlockByNumber(currentBlock)
    if e != nil {
      go c.Sync(block, syncUtility)
    } else {
      log.Errorf("Block already exists: %v", err)
      break mainloop
    }


		syncUtility.wait(c.cfg.MaxRoutines)
		syncUtility.swapChannels()

	}

	syncUtility.close(currentBlock)

	if syncUtility.synctype == "back" || syncUtility.synctype == "first" {
		c.state.syncing = false
	}

}

func (c *Crawler) Sync(block *models.Block, syncUtility Sync) {

	syncUtility.recieve()

	var uncles []*models.Uncle

	if len(block.Uncles) > 0 {
		uncles = c.GetUncles(block.Uncles, block.Number)
	}

  blockReward, uncleRewards, minted := AccumulateRewards(block, uncles)

  // TODO: replace this with a cache - iquidus
  lsb, err := c.backend.LatestSupplyBlock()
  if err != nil {
		log.Errorf("Error getting latest supply block: %v", err)
	}
  prev, _ := new(big.Int).SetString(lsb.Supply, 10)

  var supply = new(big.Int)
  supply.Add(prev, minted)

  sblock := models.Supply{Number: block.Number, Timestamp: block.Timestamp, BlockReward: blockReward.String(), UncleRewards: uncleRewards.String(), Minted: minted.String(), Supply: supply.String()}

	err = c.backend.AddSupplyBlock(sblock)
	if err != nil {
		log.Errorf("Error adding block: %v", err)
	}

	syncUtility.log(block.Number, 0, 0, 0)
	syncUtility.send(block.Number + 1)
	syncUtility.done()
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
