package config

import (
	"github.com/ubiq/spectrum-backend/api"
	"github.com/ubiq/spectrum-backend/crawler"
	"github.com/ubiq/spectrum-backend/rpc"
	"github.com/ubiq/spectrum-backend/storage"
	"github.com/ubiq/spectrum-backend/subq"
)

type Config struct {
	Threads int            `json:"threads"`
	Crawler crawler.Config `json:"crawler"`
	Subq		subq.Config		 `json:subq"`
	Mongo   storage.Config `json:"mongo"`
	Rpc     rpc.Config     `json:"rpc"`
	Api     api.Config     `json:"api"`
}

// {
//   "mongodb": {
//     "user": "spectrum",
//     "password": "UBQ4Lyfe",
//     "database": "spectrumdb",
//     "address": "localhost",
//     "port": 27017
//   },
//
//   "mongodbtest": {
//     "user": "spectrum",
//     "password": "UBQ4Lyfe",
//     "database": "spectrum-test",
//     "address": "localhost",
//     "port": 27017
//   }
// }
