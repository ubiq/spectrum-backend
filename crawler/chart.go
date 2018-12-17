package crawler

import (
	"math/big"
	"sort"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"

	"time"

	"github.com/ubiq/spectrum-backend/models"
	"github.com/ubiq/spectrum-backend/util"
)

type elem interface {
	Add(interface{})
}

type chartdata struct {
	data map[string]elem
}

func (c *chartdata) init() {
	c.data = make(map[string]elem)
}

func (c *chartdata) addElement(stamp string, element elem) {
	if c.data[stamp] == nil {
		c.data[stamp] = element
	}
	c.data[stamp].Add(element)
}

func (c *chartdata) getElement(stamp string) elem {
	return c.data[stamp]
}

func (c *chartdata) getDates() []string {
	dates := make([]string, 0)

	for k := range c.data {
		dates = append(dates, k)
	}

	sort.Slice(dates, func(i, j int) bool {
		ti, _ := time.Parse("2/01/06", dates[i])
		tj, _ := time.Parse("2/01/06", dates[j])
		return ti.Before(tj)
	})

	return dates
}

func (c *chartdata) print() {
	log.Println(c.data)
}

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

type block_chart_data struct {
	avggasprice, gaslimit, difficulty, blocktime, blocks *big.Int
}

func (b *block_chart_data) Add(bcd interface{}) {
	b.avggasprice.Add(b.avggasprice, bcd.(*block_chart_data).avggasprice)
	b.gaslimit.Add(b.gaslimit, bcd.(*block_chart_data).gaslimit)
	b.difficulty.Add(b.difficulty, bcd.(*block_chart_data).difficulty)
	b.blocktime.Add(b.blocktime, bcd.(*block_chart_data).blocktime)
	b.blocks.Add(b.blocks, bcd.(*block_chart_data).blocks)
}

func (c *Crawler) ChartBlocks() {
	var block models.Block
	var wg sync.WaitGroup
	var routines int
	var c1, c2 chan uint64

	start := time.Now()
	log.Debugf("Start block gather loop: %v", start)

	iter := c.backend.GetBlocks(0)

	data := &chartdata{}
	data.init()

	// goroutine syncing patter from block crawler

	c2 = make(chan uint64, 1)

	c2 <- 0

	c1, c2 = c2, make(chan uint64, 1)

	for iter.Next(&block) {
		wg.Add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(wg *sync.WaitGroup, b models.Block, c1 chan uint64, c2 chan uint64, data *chartdata) {
			prevStamp := <-c1
			close(c1)

			stamp := time.Unix(int64(b.Timestamp), 0).Format("2/01/06")

			avggasprice, _ := new(big.Int).SetString(b.AvgGasPrice, 10)
			gaslimit := new(big.Int).SetUint64(b.GasLimit)
			difficulty, _ := new(big.Int).SetString(b.Difficulty, 10)
			blocktime := big.NewInt(0)

			if prevStamp == 0 {
				blocktime.SetUint64(1)
			} else {
				blocktime.SetUint64(prevStamp - b.Timestamp)
			}

			em := &block_chart_data{
				avggasprice: avggasprice,
				gaslimit:    gaslimit,
				difficulty:  difficulty,
				blocktime:   blocktime,
				blocks:      big.NewInt(1),
			}

			data.addElement(stamp, em)

			c2 <- b.Timestamp
			wg.Done()
		}(&wg, block, c1, c2, data)

		c1, c2 = c2, make(chan uint64, 1)

		routines++

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

	data.print()

	if iter.Done() {

		dates := data.getDates()

		avggasprice := make([]string, 0)
		gaslimit := make([]string, 0)
		difficulty := make([]string, 0)
		hashrate := make([]string, 0)
		blocktime := make([]string, 0)

		for _, stamp := range dates {

			// Divide each for no. of blocks
			element := data.getElement(stamp).(*block_chart_data)

			avgdiff := big.NewInt(0).Div(element.difficulty, element.blocks)
			avgblocktime := big.NewFloat(0).Quo(new(big.Float).SetInt(element.blocktime), new(big.Float).SetInt(element.blocks))

			avggasprice = append(avggasprice, big.NewInt(0).Div(element.avggasprice, element.blocks).String())
			gaslimit = append(gaslimit, big.NewInt(0).Div(element.gaslimit, element.blocks).String())
			hashrate = append(hashrate, big.NewFloat(0).Quo(new(big.Float).SetInt(avgdiff), avgblocktime).String())
			blocktime = append(blocktime, util.BigFloatToString(avgblocktime, 2))
			difficulty = append(difficulty, avgdiff.String())
		}

		avggasprice_c := &models.LineChart{
			Chart:  "avggasprice",
			Labels: dates,
			Values: avggasprice,
		}

		gaslimit_c := &models.LineChart{
			Chart:  "gaslimit",
			Labels: dates,
			Values: gaslimit,
		}

		difficulty_c := &models.LineChart{
			Chart:  "difficulty",
			Labels: dates,
			Values: difficulty,
		}

		hashrate_c := &models.LineChart{
			Chart:  "hashrate",
			Labels: dates,
			Values: hashrate,
		}

		blocktime_c := &models.LineChart{
			Chart:  "blocktime",
			Labels: dates,
			Values: blocktime,
		}

		c.backend.AddLineChart(avggasprice_c)
		c.backend.AddLineChart(gaslimit_c)
		c.backend.AddLineChart(difficulty_c)
		c.backend.AddLineChart(hashrate_c)
		c.backend.AddLineChart(blocktime_c)

		log.Debugf("End blocks loop: %v", time.Since(start))
	}
}

type utility struct {
	block     int
	stamp     string
	prevstamp uint64
}

func (u *utility) rotate(ps uint64) {
	u.block--
	u.prevstamp = ps
}

func (u *utility) set(ns string) {
	u.stamp = ns
}

func (c *Crawler) ChartBlocktime() {
	var block models.Block
	var wg sync.WaitGroup
	var routines int
	var c1, c2 chan *utility

	start := time.Now()
	log.Debugf("Start blocktime gather loop: %v", start)

	iter := c.backend.GetBlocks(365)

	data := make(map[string]*big.Int)

	// goroutine syncing patter from block crawler

	c2 = make(chan *utility, 1)

	dates := make([]string, 0)

	blocktime := make([]string, 0)

	c2 <- &utility{block: 88, stamp: "", prevstamp: 0}

	c1, c2 = c2, make(chan *utility, 1)

	for iter.Next(&block) {
		wg.Add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(wg *sync.WaitGroup, b models.Block, c1 chan *utility, c2 chan *utility) {
			u := <-c1
			close(c1)

			if u.block == 88 {
				stamp := time.Unix(int64(b.Timestamp), 0).Format("2/01/06 15:04:05")
				u.set(stamp)
			}

			blocktime := big.NewInt(0)

			if u.prevstamp == 0 {
				blocktime.SetUint64(1)
			} else {
				blocktime.SetUint64(u.prevstamp - b.Timestamp)
			}

			if data[u.stamp] == nil {
				data[u.stamp] = new(big.Int)
			}

			data[u.stamp].Add(data[u.stamp], blocktime)

			if u.block == 0 {
				u = &utility{
					block:     88,
					stamp:     "",
					prevstamp: b.Timestamp,
				}
			} else {
				u.rotate(b.Timestamp)
			}

			c2 <- u
			wg.Done()
		}(&wg, block, c1, c2)

		c1, c2 = c2, make(chan *utility, 1)

		routines++

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
			ti, _ := time.Parse("2/01/06 15:04:05", dates[i])
			tj, _ := time.Parse("2/01/06 15:04:05", dates[j])
			return ti.Before(tj)
		})
		for _, v := range dates {

			avgblocktime := big.NewFloat(0).Quo(new(big.Float).SetInt(data[v]), new(big.Float).SetInt64(int64(88)))

			blocktime = append(blocktime, util.BigFloatToString(avgblocktime, 2))
		}

		blocktime := &models.LineChart{
			Chart:  "blocktime88",
			Labels: dates,
			Values: blocktime,
		}

		c.backend.AddLineChart(blocktime)

		log.Debugf("End blocks loop: %v", time.Since(start))
	}
}
