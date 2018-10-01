package models

type LineChart struct {
	Chart  string   `bson:"chart" json:"chart"`
	Labels []string `bson:"labels" json:"labels"`
	Values []string `bson:"values" json:"values"`
}
