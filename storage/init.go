package storage

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/globalsign/mgo"
	"github.com/ubiq/spectrum-backend/models"
)

func (m *MongoDB) Init() {
	store := &models.Store{
		Timestamp: time.Now().Unix(),
		Symbol:    "UBQ",
		Supply:    "36108073197716300000000000",
		Head:      1 << 62,
	}

	ss := m.db.C(models.STORE)

	if err := ss.Insert(store); err != nil {
		log.Fatalf("Could not init sysStore", err)
	}

	genesis := &models.Block{
		Number:          0,
		Timestamp:       1485633600,
		Txs:             0,
		Hash:            "0x406f1b7dd39fca54d8c702141851ed8b755463ab5b560e6f19b963b4047418af",
		ParentHash:      "0x0000000000000000000000000000000000000000000000000000000000000000",
		Sha3Uncles:      "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		Miner:           "0x3333333333333333333333333333333333333333",
		Difficulty:      "80000000000",
		TotalDifficulty: "80000000000",
		Size:            524,
		GasUsed:         0,
		GasLimit:        134217728,
		Nonce:           "0x0000000000000888",
		UncleNo:         0,
		// Empty
		BlockReward:  "0",
		UnclesReward: "0",
		AvgGasPrice:  "0",
		TxFees:       "0",
		//
		ExtraData: "0x4a756d6275636b734545",
	}

	gb := m.db.C(models.BLOCKS)

	if err := gb.Insert(genesis); err != nil {
		log.Fatalf("Could not init genesis block: %v", err)
	}

	log.Warnf("Initialized sysStore, genesis")

	m.InitIndex()

}

func (m *MongoDB) InitIndex() {

	ss := m.db.C(models.BLOCKS)

	blockno := mgo.Index{
		Key:        []string{"number"},
		Unique:     true,
		Background: true,
	}
	index := mgo.Index{
		Key:        []string{"hash"},
		Unique:     true,
		Background: true,
	}

	err := ss.EnsureIndex(blockno)
	if err != nil {
		log.Errorf("Could not init index for blocks: %v", err)
	}

	err = ss.EnsureIndex(index)
	if err != nil {
		log.Errorf("Could not init index for blocks: %v", err)
	}

	ss = m.db.C(models.REORGS)

	reorg := mgo.Index{
		Key:        []string{"hash"},
		Unique:     true,
		Background: true,
	}

	err = ss.EnsureIndex(reorg)
	if err != nil {
		log.Errorf("Could not init index for reorgs: %v", err)
	}

	ss = m.db.C(models.UNCLES)

	uncle := mgo.Index{
		Key:        []string{"hash"},
		Unique:     true,
		Background: true,
	}

	err = ss.EnsureIndex(uncle)
	if err != nil {
		log.Errorf("Could not init index for uncles: %v", err)
	}

	ss = m.db.C(models.TXNS)

	block := mgo.Index{
		Key:        []string{"number"},
		Background: true,
	}

	/* Index already defined for blocks */

	from := mgo.Index{
		Key:        []string{"from"},
		Background: true,
	}
	to := mgo.Index{
		Key:        []string{"to"},
		Background: true,
	}
	contractAddress := mgo.Index{
		Key:        []string{"contractAddress"},
		Background: true,
	}

	err = ss.EnsureIndex(block)
	if err != nil {
		log.Errorf("Could not init index for transactions: %v", err)
	}
	err = ss.EnsureIndex(index)
	if err != nil {
		log.Errorf("Could not init index for transactions: %v", err)
	}
	err = ss.EnsureIndex(from)
	if err != nil {
		log.Errorf("Could not init index for transactions: %v", err)
	}
	err = ss.EnsureIndex(to)
	if err != nil {
		log.Errorf("Could not init index for transactions: %v", err)
	}
	err = ss.EnsureIndex(contractAddress)
	if err != nil {
		log.Errorf("Could not init index for transactions: %v", err)
	}

	ss = m.db.C(models.TRANSFERS)

	/* Using the ones defined for regular transactions */

	err = ss.EnsureIndex(block)
	if err != nil {
		log.Errorf("Could not init index for tokentransfers: %v", err)
	}
	err = ss.EnsureIndex(index)
	if err != nil {
		log.Errorf("Could not init index for tokentransfers: %v", err)
	}
	err = ss.EnsureIndex(from)
	if err != nil {
		log.Errorf("Could not init index for tokentransfers: %v", err)
	}
	err = ss.EnsureIndex(to)
	if err != nil {
		log.Errorf("Could not init index for tokentransfers: %v", err)
	}

	log.Warnf("Intialized database indexes")

}