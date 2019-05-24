package models

type Supply struct {
	Number       uint64 `bson:"number" json:"number"`
	Timestamp    uint64 `bson:"timestamp" json:"timestamp"`
	BlockReward  string `bson:"blockReward" json:"blockReward"`
	UncleRewards string `bson:"uncleRewards" json:"uncleRewards"`
  Minted       string `bson:"minted" json:"minted"`
	Supply       string `bson:"supply" json:"supply"`
}
