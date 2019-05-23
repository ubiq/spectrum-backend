package models

type Supply struct {
	Number       uint64 `bson:"number" json:"number"`
	Timestamp    uint64 `bson:"timestamp" json:"timestamp"`
	BlockReward  string `bson:"blockReward" json:"blockReward"`
	UnclesReward string `bson:"unclesReward" json:"unclesReward"`
  Minted       string `bson:"minted" json:"minted"`
	Supply       string `bson:"supply" json:"supply"`
}
