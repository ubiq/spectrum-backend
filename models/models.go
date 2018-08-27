package models

const (
	BLOCKS    = "blocks"
	TXNS      = "transactions"
	UNCLES    = "uncles"
	TRANSFERS = "tokentransfers"
	REORGS    = "forkedblocks"
	STORE     = "sysstores"
)

type Store struct {
	Timestamp   int64  `bson:"timestamp" json:"timestamp"`
	Symbol      string `bson:"symbol" json:"symbol"`
	Supply      string `bson:"supply" json:"supply"`
	LatestBlock *Block `bson:"latestBlock" json:"latestBlock"`
	Price       string `bson:"price" json:"price"`
}
