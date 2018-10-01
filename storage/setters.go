package storage

import (
	"github.com/globalsign/mgo/bson"
	"github.com/ubiq/spectrum-backend/models"
)

func (m *MongoDB) AddTransaction(tx *models.Transaction) error {
	// start := time.Now()
	ss := m.db.C(models.TXNS)

	if err := ss.Insert(tx); err != nil {
		return err
	}
	// log.Debugf("AddTransaction: %v", time.Since(start))
	return nil
}

func (m *MongoDB) AddTokenTransfer(tt *models.TokenTransfer) error {
	// start := time.Now()
	ss := m.db.C(models.TRANSFERS)

	if err := ss.Insert(tt); err != nil {
		return err
	}
	// log.Debugf("AddTokenTransfer: %v", time.Since(start))
	return nil
}

func (m *MongoDB) AddUncle(u *models.Uncle) error {
	// start := time.Now()
	ss := m.db.C(models.UNCLES)

	if err := ss.Insert(u); err != nil {
		return err
	}
	// log.Debugf("AddUncle: %v", time.Since(start))
	return nil
}

func (m *MongoDB) AddBlock(b *models.Block) error {
	// start := time.Now()
	ss := m.db.C(models.BLOCKS)

	if err := ss.Insert(b); err != nil {
		return err
	}
	// log.Debugf("AddBlock: %v", time.Since(start))
	return nil
}

func (m *MongoDB) AddForkedBlock(b *models.Block) error {
	// start := time.Now()
	ss := m.db.C(models.REORGS)

	if err := ss.Insert(b); err != nil {
		return err
	}
	// log.Debugf("AddForkedBlock: %v", time.Since(start))
	return nil
}

func (m *MongoDB) AddLineChart(t *models.LineChart) error {
	// start := time.Now()
	ss := m.db.C(models.CHARTS)

	if _, err := ss.Upsert(bson.M{"chart": t.Chart}, t); err != nil {
		return err
	}
	// log.Debugf("AddTxnsChartData: %v", time.Since(start))
	return nil
}
