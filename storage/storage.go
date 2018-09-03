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

func (m *MongoDB) IndexHead() uint64 {
	var store models.Store

	err := m.db.C(models.STORE).Find(&bson.M{}).Limit(1).One(&store)

	if err != nil {
		log.Fatalf("Error during initialization: %v", err)
	}

	return store.Head
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

	if latestBlock.Number < head {
		head = latestBlock.Number
	}

	err = m.db.C(models.STORE).Update(&bson.M{}, &bson.M{"timestamp": time.Now().Unix(), "supply": new_supply.String(), "symbol": "UBQ", "price": price, "latestBlock": latestBlock, "head": head})

	if err != nil {
		return err
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

func (m *MongoDB) Purge(height uint64) error {
	selector := &bson.M{"number": height}

	err := m.db.C(models.BLOCKS).Remove(selector)
	if err != nil {
		return err
	}

	err = m.db.C(models.TXNS).Remove(selector)
	if err != nil {
		return err
	}

	err = m.db.C(models.TRANSFERS).Remove(selector)
	if err != nil {
		return err
	}

	err = m.db.C(models.UNCLES).Remove(selector)
	if err != nil {
		return err
	}

	return nil
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
