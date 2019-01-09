package storage

import (
	"math/big"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	log "github.com/sirupsen/logrus"
	"github.com/ubiq/spectrum-backend/models"
)

type Config struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Address  string `json:"address"`
}

type MongoDB struct {
	session *mgo.Session
	db      *mgo.Database
}

func NewConnection(cfg *Config) (*MongoDB, error) {
	session, err := mgo.DialWithInfo(&mgo.DialInfo{
		Addrs:    []string{cfg.Address},
		Database: cfg.Database,
		Username: cfg.User,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, err
	}
	return &MongoDB{session, session.DB("")}, nil
}

func (m *MongoDB) IsFirstRun() bool {
	var store models.Store

	err := m.db.C(models.STORE).Find(&bson.M{}).Limit(1).One(&store)

	if err != nil {
		if err.Error() == "not found" {
			return true
		} else {
			log.Fatalf("Error during initialization: %v", err)
		}
	}

	return false
}

func (m *MongoDB) IndexHead() [1]uint64 {
	var store models.Store

	err := m.db.C(models.STORE).Find(&bson.M{}).Limit(1).One(&store)

	if err != nil {
		log.Fatalf("Error during initialization: %v", err)
	}

	return store.Sync
}

func (m *MongoDB) latestStoredBlock() uint64 {
	var block models.Block

	err := m.db.C(models.BLOCKS).Find(bson.M{}).Sort("-number").Limit(1).One(&block)

	if err != nil {
		log.Errorf("latestStoredBlock: error querying db: %v", err)
	}

	return block.Number
}

// TODO: eventually move this to methods for Systore

func (m *MongoDB) UpdateStore(latestBlock *models.Block, synctype string /* , minted string, price string, forkedBlock bool */) error {

	head := m.IndexHead()

	switch synctype {

	// This case will fire when initial sync is complete and it's just adding blocks as they come in
	case "top":

		// If the block behind is present the sync reached the top of the db

		if m.IsPresent(latestBlock.Number - 1) {
			head = [1]uint64{0}
		} else {
			head[0] = latestBlock.Number
		}

		err := m.db.C(models.STORE).Update(&bson.M{"symbol": "sync"}, &bson.M{"symbol": "sync", "sync": head})

		if err != nil {
			return err
		}

		// This case will fire when there is a sync active and the sync variable is being used by another crawler routine
	case "":

		// Setting it to 1 << 62 because omitting the field in the update method makes the key disappear instead of not updating it
		// 1<< 62 is greater than any blocknumber so next case will always trigger

		err := m.db.C(models.STORE).Update(&bson.M{"symbol": "sync"}, &bson.M{"symbol": "sync", "sync": [1]uint64{1 << 62}})

		if err != nil {
			return err
		}

		// This case will fire when it's syncing backwards
	case "back", "first":

		// To check if we're at the top of the db we check one block behind

		if m.IsPresent(latestBlock.Number - 1) {
			head = [1]uint64{0}
		} else {
			head[0] = latestBlock.Number
		}

		err := m.db.C(models.STORE).Update(&bson.M{"symbol": "sync"}, &bson.M{"symbol": "sync", "sync": head})

		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MongoDB) GetSupply(symbol string) (*big.Int, error) {
	var store models.Store

	err := m.db.C(models.STORE).Find(&bson.M{"symbol": symbol}).One(&store)

	if err != nil {
		return big.NewInt(0), err
	}

	x := big.NewInt(0)

	x.SetString(store.Supply, 10)

	return x, nil
}

func (m *MongoDB) UpdateSupply(ticker string, new *models.Store) error {

	err := m.db.C(models.STORE).Update(&bson.M{"symbol": ticker}, new)

	if err != nil {
		return err
	}

	return nil
}

func (m *MongoDB) GetBlock(height uint64) (*models.Block, error) {
	var block models.Block

	err := m.db.C(models.BLOCKS).Find(&bson.M{"number": height}).Limit(1).One(&block)

	if err != nil {
		return &models.Block{}, err
	}

	return &block, nil
}

func (m *MongoDB) Purge(height uint64) {

	// TODO: make this better

	selector := &bson.M{"blockNumber": height}
	blockselector := &bson.M{"number": height}

	bulk := m.db.C(models.TXNS).Bulk()
	bulk.RemoveAll(selector)
	_, err := bulk.Run()
	if err != nil {
		log.Errorf("Error purging transactions: %v", err)
	}

	bulk = m.db.C(models.TRANSFERS).Bulk()
	bulk.RemoveAll(selector)
	_, err = bulk.Run()
	if err != nil {
		log.Errorf("Error purging token transfers: %v", err)
	}

	bulk = m.db.C(models.UNCLES).Bulk()
	bulk.RemoveAll(selector)
	_, err = bulk.Run()
	if err != nil {
		log.Errorf("Error purging uncles: %v", err)
	}

	bulk = m.db.C(models.BLOCKS).Bulk()
	bulk.RemoveAll(blockselector)
	_, err = bulk.Run()
	if err != nil {
		log.Errorf("Error purging blocks: %v", err)
	}

}

func (m *MongoDB) Ping() error {
	return m.session.Ping()
}

func (m *MongoDB) IsPresent(height uint64) bool {

	if height == 0 {
		return true
	}

	if dbHead := m.latestStoredBlock(); dbHead == height {
		return false
	}

	var rbn models.RawBlockDetails
	err := m.db.C(models.BLOCKS).Find(&bson.M{"number": height}).Limit(1).One(&rbn)

	if err != nil {
		if err.Error() == "not found" {
			return false
		} else {
			log.Errorf("Error checking for block in db: %v", err)
		}
	}

	return true
}

func (m *MongoDB) IsInDB(height uint64, hash string) (bool, bool) {
	var rbn models.RawBlockDetails
	err := m.db.C(models.BLOCKS).Find(&bson.M{"number": height}).Limit(1).One(&rbn)

	if err != nil {
		if err.Error() == "not found" {
			return false, false
		} else {
			log.Errorf("Error checking for block in db: %v", err)
		}
	}

	if _, contendentHash := rbn.Convert(); contendentHash != hash {
		return true, true
	}

	return true, false
}
