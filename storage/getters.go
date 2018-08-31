package storage

import (
	"github.com/globalsign/mgo/bson"
	"github.com/ubiq/spectrum-backend/models"
)

func (m *MongoDB) BlockByNumber(number uint64) (models.Block, error) {
	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{"number": number}).One(&block)
	return block, err
}

func (m *MongoDB) BlockByHash(hash string) (models.Block, error) {
	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{"hash": hash}).One(&block)
	return block, err
}

func (m *MongoDB) LatestBlock() (models.Block, error) {
	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{}).Sort("-number").Limit(1).One(&block)
	return block, err
}

func (m *MongoDB) Store() (models.Store, error) {
	var store models.Store
	err := m.db.C(models.STORE).Find(bson.M{}).Limit(1).One(&store)
	return store, err
}

func (m *MongoDB) LatestBlocks(limit int) ([]models.Block, error) {
	var blocks []models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{}).Sort("-number").Limit(limit).All(&blocks)
	return blocks, err
}

func (m *MongoDB) LatestUncles(limit int) ([]models.Uncle, error) {
	var uncles []models.Uncle
	err := m.db.C(models.UNCLES).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&uncles)
	return uncles, err
}

func (m *MongoDB) LatestForkedBlocks(limit int) ([]models.Block, error) {
	var blocks []models.Block
	err := m.db.C(models.REORGS).Find(bson.M{}).Sort("-number").Limit(limit).All(&blocks)
	return blocks, err
}

func (m *MongoDB) TransactionByHash(hash string) (models.Transaction, error) {
	var txn models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{"hash": hash}).One(&txn)
	return txn, err
}

func (m *MongoDB) UncleByHash(hash string) (models.Uncle, error) {
	var uncle models.Uncle
	err := m.db.C(models.UNCLES).Find(bson.M{"hash": hash}).One(&uncle)
	return uncle, err
}

func (m *MongoDB) LatestTransactions(limit int) ([]models.Transaction, error) {
	var txns []models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&txns)
	return txns, err
}

func (m *MongoDB) LatestTransactionsByAccount(hash string) ([]models.Transaction, error) {
	var txns []models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Sort("-blockNumber").Limit(25).All(&txns)
	return txns, err
}

func (m *MongoDB) LatestTokenTransfersByAccount(hash string) ([]models.TokenTransfer, error) {
	var transfers []models.TokenTransfer
	err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Sort("-blockNumber").Limit(25).All(&transfers)
	return transfers, err
}

func (m *MongoDB) LatestTokenTransfers(limit int) ([]models.TokenTransfer, error) {
	var transfers []models.TokenTransfer
	err := m.db.C(models.TRANSFERS).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&transfers)
	return transfers, err
}

func (m *MongoDB) TxnCount(hash string) (int, error) {
	count, err := m.db.C(models.TXNS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Count()
	return count, err
}

func (m *MongoDB) TotalTxnCount() (int, error) {
	count, err := m.db.C(models.TXNS).Find(bson.M{}).Count()
	return count, err
}

func (m *MongoDB) TokenTransferCount(hash string) (int, error) {
	count, err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Count()
	return count, err
}

func (m *MongoDB) TotalBlockCount() (int, error) {
	count, err := m.db.C(models.BLOCKS).Find(bson.M{}).Count()
	return count, err
}

func (m *MongoDB) TotalUncleCount() (int, error) {
	count, err := m.db.C(models.UNCLES).Find(bson.M{}).Count()
	return count, err
}
