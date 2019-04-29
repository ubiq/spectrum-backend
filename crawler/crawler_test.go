package crawler_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ubiq/spectrum-backend/config"
	"github.com/ubiq/spectrum-backend/crawler"
	"github.com/ubiq/spectrum-backend/crawler/mocks"
)

func BenchmarkFib10(b *testing.B) {

	var cfg config.Config

	rawjson := bytes.NewBufferString(`{
  "threads": 4,
  "crawler": {
    "enabled": true,
    "interval": "5s",
    "routines": 5
  },
  "api": {
    "enabled": false,
    "port": "3000"
  },
  "mongo": {
    "address": "127.0.0.1:27017",
    "database": "spectrum-test",
    "user": "spectrum",
    "password": "UBQ4Lyfe"
  },
  "rpc": {
    "url": "http://127.0.0.1:8588",
    "timeout": "60s"
  }
}`)

	json.NewDecoder(rawjson).Decode(&cfg)

	cr := crawler.New(&mocks.Database{}, &mocks.RPCClient{}, &cfg.Crawler)

	sync_t := crawler.NewSync()

	for n := 0; n < b.N; n++ {
		block, _ := cr.rpc.GetBlockByHeight(n)

		crawler.Sync(block)

	}
}
