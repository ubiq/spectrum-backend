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

func (m *MongoDB) TransactionByContractAddress(hash string) (models.Transaction, error) {
	var txn models.Transaction

	err := m.db.C(models.TXNS).Find(bson.M{"contractAddress": hash}).One(&txn)
	return txn, err
}

func (m *MongoDB) LatestTransfersByToken(hash string) ([]models.TokenTransfer, error) {
	var transfers []models.TokenTransfer

	err := m.db.C(models.TRANSFERS).Find(bson.M{"contract": hash}).Sort("-blockNumber").Limit(1000).All(&transfers)
	return transfers, err
}

func (m *MongoDB) TokenTransferCountByContract(hash string) (int, error) {
	count, err := m.db.C(models.TRANSFERS).Find(bson.M{"contract": hash}).Count()
	return count, err
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

func (m *MongoDB) ChartData(chart string, limit int64) (models.LineChart, error) {
	var chartData models.LineChart

	err := m.db.C(models.CHARTS).Find(bson.M{"chart": chart}).One(&chartData)

	if err != nil {
		return models.LineChart{}, err
	}

	if limit >= int64(len(chartData.Labels)) || limit >= int64(len(chartData.Values)) || limit == 0 {
		limit = int64(len(chartData.Labels) - 1)
	}

	// Limit selects items from the end of the slice; we exclude the last element (current day)
	// TODO: Eventually fix this in the iterators

	chartData.Labels = chartData.Labels[int64(len(chartData.Labels)-1)-limit : len(chartData.Labels)-1]
	chartData.Values = chartData.Values[int64(len(chartData.Values)-1)-limit : len(chartData.Values)-1]

	return chartData, err
}

func (m *MongoDB) TxnCount(hash string) (int, error) {
	count, err := m.db.C(models.TXNS).Find(bson.M{"$or": []bson.M{bson.M{"from": hash}, bson.M{"to": hash}}}).Count()
	return count, err
}

func (m *MongoDB) TotalBlockCount() (int, error) {
	count, err := m.db.C(models.BLOCKS).Find(bson.M{}).Count()

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

func (m *MongoDB) TokenTransfersByAccount(token string, account string) ([]models.TokenTransfer, error) {
	var transfers []models.TokenTransfer

	err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"$and": []bson.M{bson.M{"from": account}, bson.M{"contract": token}}}, bson.M{"$and": []bson.M{bson.M{"to": account}, bson.M{"contract": token}}}}}).Sort("-blockNumber").All(&transfers)
	return transfers, err
}

func (m *MongoDB) TokenTransferByAccountCount(token string, account string) (int, error) {
	count, err := m.db.C(models.TRANSFERS).Find(bson.M{"$or": []bson.M{bson.M{"$and": []bson.M{bson.M{"from": account}, bson.M{"contract": token}}}, bson.M{"$and": []bson.M{bson.M{"to": account}, bson.M{"contract": token}}}}}).Count()
	return count, err
}

func (m *MongoDB) TotalUncleCount() (int, error) {
	count, err := m.db.C(models.UNCLES).Find(bson.M{}).Count()
	return count, err
}
