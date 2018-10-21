package storage

import (
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/ubiq/spectrum-backend/models"
)

var EOD = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.UTC)

func (m *MongoDB) GetTxnCounts(days int) *mgo.Iter {
	var from, to int64

	hours, _ := time.ParseDuration("-23h59m59s")

	if days == 0 {
		from = 1485633600
		to = EOD.Unix()
	} else {
		from = EOD.Add(hours).AddDate(0, 0, -days).Unix()
		to = EOD.Unix()
	}

	pipeline := []bson.M{{"$match": bson.M{"timestamp": bson.M{"$gte": from, "$lt": to}}}}

	pipe := m.db.C(models.TXNS).Pipe(pipeline)

	return pipe.Iter()

}

func (m *MongoDB) GetBlocks(days int) *mgo.Iter {
	// genesis block: 1485633600
	var from, to int64

	hours, _ := time.ParseDuration("-23h59m59s")

	if days == 0 {
		from = 1485633600
		to = EOD.Unix()
	} else {
		from = EOD.Add(hours).AddDate(0, 0, -days).Unix()
		to = EOD.Unix()
	}

	pipeline := []bson.M{{"$match": bson.M{"timestamp": bson.M{"$gte": from, "$lt": to}}}}

	pipe := m.db.C(models.BLOCKS).Pipe(pipeline)

	return pipe.Iter()

}
