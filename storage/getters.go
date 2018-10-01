package storage

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/globalsign/mgo/bson"
	"github.com/ubiq/spectrum-backend/models"
)

func (m *MongoDB) BlockByNumber(number uint64) (models.Block, error) {
	start := time.Now()

	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{"number": number}).One(&block)
	log.Debugf("BlockByNumber: %v", time.Since(start))
	return block, err
}

func (m *MongoDB) BlockByHash(hash string) (models.Block, error) {
	start := time.Now()

	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{"hash": hash}).One(&block)
	log.Debugf("BlockByHash: %v", time.Since(start))
	return block, err
}

func (m *MongoDB) LatestBlock() (models.Block, error) {
	start := time.Now()

	var block models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{}).Sort("-number").Limit(1).One(&block)
	log.Debugf("LatestBlock: %v", time.Since(start))
	return block, err
}

func (m *MongoDB) Store() (models.Store, error) {
	start := time.Now()

	var store models.Store
	err := m.db.C(models.STORE).Find(bson.M{}).Limit(1).One(&store)
	log.Debugf("Store: %v", time.Since(start))
	return store, err
}

func (m *MongoDB) LatestBlocks(limit int) ([]models.Block, error) {
	start := time.Now()

	var blocks []models.Block
	err := m.db.C(models.BLOCKS).Find(bson.M{}).Sort("-number").Limit(limit).All(&blocks)
	log.Debugf("LatestBlocks: %v", time.Since(start))
	return blocks, err
}

func (m *MongoDB) LatestUncles(limit int) ([]models.Uncle, error) {
	start := time.Now()

	var uncles []models.Uncle
	err := m.db.C(models.UNCLES).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&uncles)
	log.Debugf("LatestUncles: %v", time.Since(start))
	return uncles, err
}

func (m *MongoDB) LatestForkedBlocks(limit int) ([]models.Block, error) {
	start := time.Now()

	var blocks []models.Block
	err := m.db.C(models.REORGS).Find(bson.M{}).Sort("-number").Limit(limit).All(&blocks)
	log.Debugf("LatestForkedBlocks: %v", time.Since(start))
	return blocks, err
}

func (m *MongoDB) TransactionByHash(hash string) (models.Transaction, error) {
	start := time.Now()

	var txn models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{"hash": hash}).One(&txn)
	log.Debugf("TransactionByHash: %v", time.Since(start))
	return txn, err
}

func (m *MongoDB) UncleByHash(hash string) (models.Uncle, error) {
	start := time.Now()

	var uncle models.Uncle
	err := m.db.C(models.UNCLES).Find(bson.M{"hash": hash}).One(&uncle)
	log.Debugf("UncleByHash: %v", time.Since(start))
	return uncle, err
}

func (m *MongoDB) LatestTransactions(limit int) ([]models.Transaction, error) {
	start := time.Now()

	var txns []models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&txns)
	log.Debugf("LatestTransactions: %v", time.Since(start))
	return txns, err
}

func (m *MongoDB) LatestTransactionsByAccount(hash string) ([]models.Transaction, error) {
	start := time.Now()

	var txns []models.Transaction
	err := m.db.C(models.TXNS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Sort("-blockNumber").Limit(25).All(&txns)
	log.Debugf("LatestTransactionsByAccount: %v", time.Since(start))
	return txns, err
}

func (m *MongoDB) LatestTokenTransfersByAccount(hash string) ([]models.TokenTransfer, error) {
	start := time.Now()

	var transfers []models.TokenTransfer
	err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Sort("-blockNumber").Limit(25).All(&transfers)
	log.Debugf("LatestTokenTransfersByAccount: %v", time.Since(start))
	return transfers, err
}

func (m *MongoDB) LatestTokenTransfers(limit int) ([]models.TokenTransfer, error) {
	start := time.Now()

	var transfers []models.TokenTransfer
	err := m.db.C(models.TRANSFERS).Find(bson.M{}).Sort("-blockNumber").Limit(limit).All(&transfers)
	log.Debugf("LatestTokenTransfers: %v", time.Since(start))
	return transfers, err
}

func (m *MongoDB) ChartData(chart string, limit int64) (models.LineChart, error) {
	start := time.Now()

	var chartData models.LineChart

	err := m.db.C(models.CHARTS).Find(bson.M{"chart": chart}).One(&chartData)
	log.Debugf("ChartData: %v", time.Since(start))

	if limit >= int64(len(chartData.Labels)) || limit >= int64(len(chartData.Values)) {
		limit = int64(len(chartData.Labels))
	}

	chartData.Labels = chartData.Labels[int64(len(chartData.Labels))-limit:]
	chartData.Values = chartData.Values[int64(len(chartData.Values))-limit:]

	return chartData, err
}

func (m *MongoDB) TxnCount(hash string) (int, error) {
	start := time.Now()

	count, err := m.db.C(models.TXNS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Count()
	log.Debugf("TxnCount: %v", time.Since(start))

	return count, err
}

func (m *MongoDB) TotalTxnCount() (int, error) {
	start := time.Now()

	count, err := m.db.C(models.TXNS).Find(bson.M{}).Count()
	log.Debugf("TotalTxnCount: %v", time.Since(start))

	return count, err
}

func (m *MongoDB) TokenTransferCount(hash string) (int, error) {
	start := time.Now()

	count, err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Count()
	log.Debugf("TokenTransferCount: %v", time.Since(start))

	return count, err
}

func (m *MongoDB) TotalBlockCount() (int, error) {
	start := time.Now()

	count, err := m.db.C(models.BLOCKS).Find(bson.M{}).Count()
	log.Debugf("TotalBlockCount: %v", time.Since(start))

	return count, err
}

func (m *MongoDB) TotalUncleCount() (int, error) {
	start := time.Now()

	count, err := m.db.C(models.UNCLES).Find(bson.M{}).Count()
	log.Debugf("TotalUncleCount: %v", time.Since(start))

	return count, err
}
