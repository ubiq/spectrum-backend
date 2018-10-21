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
	var wg sync.WaitGroup
	var routines int

	start := time.Now()
	log.Debugf("Start txns gather loop: %v", start)

	iter := c.backend.GetTxnCounts(0)

	data := make(map[string]int)

	// goroutine syncing patter from block crawler

	c1, c2 := make(chan struct{}, 1), make(chan struct{}, 1)

	dates := make([]string, 0)
	values := make([]string, 0)

	c2 <- struct{}{}

	c1, c2 = c2, make(chan struct{}, 1)

	for iter.Next(&transaction) {
		wg.Add(1)

		// Transaction is passed by value since each iteration unmarshals a new tx into "transaction"

		go func(wg *sync.WaitGroup, tx models.Transaction, c1 chan struct{}, c2 chan struct{}) {
			<-c1
			close(c1)

			stamp := time.Unix(int64(tx.Timestamp), 0).Format("2/01/06")

			data[stamp] += 1

			c2 <- struct{}{}

			wg.Done()
		}(&wg, transaction, c1, c2)

		routines++
		c1, c2 = c2, make(chan struct{}, 1)

		if routines == 10 {
			wg.Wait()
		}
	}

	<-c1
	close(c1)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {

		for k, _ := range data {
			dates = append(dates, k)
		}

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
		log.Debugf("End txns loop: %v", time.Since(start))

	}
}

func (c *Crawler) ChartBlocks() {
	var block models.Block
	var wg sync.WaitGroup
	var routines int

	start := time.Now()
	log.Debugf("Start block gather loop: %v", start)

	iter := c.backend.GetBlocks(0)

	data := make(map[string][]*big.Int)

	// goroutine syncing patter from block crawler

	c1, c2 := make(chan struct{}, 1), make(chan struct{}, 1)

	dates := make([]string, 0)

	avggasprice := make([]string, 0)
	gaslimit := make([]string, 0)
	difficulty := make([]string, 0)

	c2 <- struct{}{}

	c1, c2 = c2, make(chan struct{}, 1)

	for iter.Next(&block) {
		wg.Add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(wg *sync.WaitGroup, b models.Block, c1 chan struct{}, c2 chan struct{}) {
			<-c1
			close(c1)

			stamp := time.Unix(int64(b.Timestamp), 0).Format("2/01/06")

			avggasprice := big.NewInt(0)
			gaslimit := big.NewInt(0)
			difficulty := big.NewInt(0)

			avggasprice.SetString(b.AvgGasPrice, 10)
			gaslimit.SetUint64(b.GasLimit)
			difficulty.SetString(b.Difficulty, 10)

			if data[stamp] == nil {
				data[stamp] = make([]*big.Int, 3)
				data[stamp][0] = big.NewInt(0)
				data[stamp][1] = big.NewInt(0)
				data[stamp][2] = big.NewInt(0)
			}
			data[stamp][0].Add(data[stamp][0], avggasprice)
			data[stamp][1].Add(data[stamp][1], gaslimit)
			data[stamp][2].Add(data[stamp][2], difficulty)

			c2 <- struct{}{}
			wg.Done()
		}(&wg, block, c1, c2)

		routines++
		c1, c2 = c2, make(chan struct{}, 1)

		if routines == 10 {
			wg.Wait()
		}
	}

	// Wait for loop to end

	<-c1
	close(c1)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {

		for k, _ := range data {
			dates = append(dates, k)
		}

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

		log.Debugf("End blocks loop: %v", time.Since(start))
	}
}
