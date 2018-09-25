package storage

import (
	"math/big"
	"time"

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

func (m *MongoDB) UpdateStore(latestBlock *models.Block, minted string, price string, forkedBlock bool) error {

	x := big.NewInt(0)
	x.SetString(minted, 10)

	new_supply, err := m.GetSupply()

	if err != nil {
		return err
	}

	switch forkedBlock {
	case true:
		new_supply.Sub(new_supply, x)
	case false:
		new_supply.Add(new_supply, x)
	}

	head := m.IndexHead()

	switch {
	case len(head) == 0:

		head[0] = latestBlock.Number

		// TODO: Figure out if latestblock here can be removed
		err = m.db.C(models.STORE).Update(&bson.M{}, &bson.M{"timestamp": time.Now().Unix(), "supply": new_supply.String(), "symbol": "UBQ", "price": price, "latestBlock": latestBlock, "sync": head})

		if err != nil {
			return err
		}
	case latestBlock.Number < head[0]:
		// To check if we're at the top of the db we check one block behind
		// due to the nature of the IsPresent() function and how we detect reorgs
		if m.IsPresent(latestBlock.Number - 1) {
			head = [1]uint64{}
		} else {
			head[0] = latestBlock.Number
		}

		// TODO: Figure out if latestblock here can be removed
		err = m.db.C(models.STORE).Update(&bson.M{}, &bson.M{"timestamp": time.Now().Unix(), "supply": new_supply.String(), "symbol": "UBQ", "price": price, "latestBlock": latestBlock, "sync": head})

		if err != nil {
			return err
		}
	case latestBlock.Number > head[0]:

		err = m.db.C(models.STORE).Update(&bson.M{}, &bson.M{"timestamp": time.Now().Unix(), "supply": new_supply.String(), "symbol": "UBQ", "price": price, "latestBlock": latestBlock})

		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MongoDB) GetSupply() (*big.Int, error) {
	var store models.Store

	err := m.db.C(models.STORE).Find(&bson.M{}).Limit(1).One(&store)

	if err != nil {
		return big.NewInt(0), err
	}

	x := big.NewInt(0)

	x.SetString(store.Supply, 10)

	return x, nil
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

	var rbn models.RawBlockDetails
	err := m.db.C(models.BLOCKS).Find(&bson.M{"number": height}).Limit(1).One(&rbn)

	if err != nil {
		if err.Error() == "not found" {
			return false
		} else {
			log.Errorf("Error checking for block in db: %v", err)
		}
	}

	if number, _ := rbn.Convert(); number == height {
		return false
	}

	return true
}

func (m *MongoDB) IsForkedBlock(height uint64, hash string) bool {
	var rbn models.RawBlockDetails
	err := m.db.C(models.BLOCKS).Find(&bson.M{"number": height}).Limit(1).One(&rbn)

	if err != nil {
		if err.Error() == "not found" {
			return false
		} else {
			log.Errorf("Error checking for block in db: %v", err)
		}
	}

	if bn, contendentHash := rbn.Convert(); bn == height && contendentHash != hash {
		return true
	}

	return false
}
