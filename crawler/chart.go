package crawler

import (
	"math/big"
	"sort"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"

	"time"

	"github.com/ubiq/spectrum-backend/models"
)

const DAYS = 14

// Functions here are used to iterate through different objects and extract chart data

func (c *Crawler) ChartTxns() {
	var transaction models.Transaction
	var mu sync.Mutex
	var wg sync.WaitGroup
	var routines int

	iter := c.backend.GetTxnCounts(0)

	data := make(map[string]int)

	dates := make([]string, 0)
	values := make([]string, 0)

	for iter.Next(&transaction) {
		wg.Add(1)

		// Transaction is passed by value since each iteration unmarshals a new tx into "transaction"

		go func(mu *sync.Mutex, wg *sync.WaitGroup, tx models.Transaction) {
			stamp := time.Unix(int64(tx.Timestamp), 0).Format("2/01/06")

			mu.Lock()
			data[stamp] += 1
			mu.Unlock()

			wg.Done()
		}(&mu, &wg, transaction)

		routines++

		if routines == 10 {
			wg.Wait()
		}
	}

	mu.Lock()
	for k, _ := range data {
		dates = append(dates, k)
	}
	mu.Unlock()

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {
		sort.Slice(dates, func(i, j int) bool {
			ti, _ := time.Parse("2/01/06", dates[i])
			tj, _ := time.Parse("2/01/06", dates[j])
			return ti.Before(tj)
		})
		for _, v := range dates {
			s := strconv.FormatInt(int64(data[v]), 10)
			values = append(values, s)
		}

		doc := &models.LineChart{
			Chart:  "txns",
			Labels: dates,
			Values: values,
		}

		c.backend.AddLineChart(doc)
	}
}

func (c *Crawler) ChartBlocks() {
	var block models.Block
	var mu sync.Mutex
	var wg sync.WaitGroup
	var routines int

	iter := c.backend.GetBlocks(0)

	data := make(map[string][]*big.Int)

	dates := make([]string, 0)

	avggasprice := make([]string, 0)
	gaslimit := make([]string, 0)
	difficulty := make([]string, 0)

	for iter.Next(&block) {
		wg.Add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(mu *sync.Mutex, wg *sync.WaitGroup, b models.Block) {
			stamp := time.Unix(int64(b.Timestamp), 0).Format("2/01/06")

			avggasprice := big.NewInt(0)
			gaslimit := big.NewInt(0)
			difficulty := big.NewInt(0)

			avggasprice.SetString(b.AvgGasPrice, 10)
			gaslimit.SetUint64(b.GasLimit)
			difficulty.SetString(b.Difficulty, 10)

			mu.Lock()
			if data[stamp] == nil {
				data[stamp] = make([]*big.Int, 3)
				data[stamp][0] = big.NewInt(0)
				data[stamp][1] = big.NewInt(0)
				data[stamp][2] = big.NewInt(0)
			}
			data[stamp][0].Add(data[stamp][0], avggasprice)
			data[stamp][1].Add(data[stamp][1], gaslimit)
			data[stamp][2].Add(data[stamp][2], difficulty)
			mu.Unlock()

			wg.Done()
		}(&mu, &wg, block)

		routines++

		if routines == 10 {
			wg.Wait()
		}
	}

	for k, _ := range data {
		dates = append(dates, k)
	}

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {
		sort.Slice(dates, func(i, j int) bool {
			ti, _ := time.Parse("2/01/06", dates[i])
			tj, _ := time.Parse("2/01/06", dates[j])
			return ti.Before(tj)
		})
		for _, v := range dates {
			avggasprice = append(avggasprice, data[v][0].String())
			gaslimit = append(gaslimit, data[v][1].String())
			difficulty = append(difficulty, data[v][2].String())
		}

		avggasprice := &models.LineChart{
			Chart:  "avggasprice",
			Labels: dates,
			Values: avggasprice,
		}

		gaslimit := &models.LineChart{
			Chart:  "gaslimit",
			Labels: dates,
			Values: gaslimit,
		}

		difficulty := &models.LineChart{
			Chart:  "difficulty",
			Labels: dates,
			Values: difficulty,
		}

		c.backend.AddLineChart(avggasprice)
		c.backend.AddLineChart(gaslimit)
		c.backend.AddLineChart(difficulty)
	}
}
