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

	start := time.Now()
	log.Debugf("Start txns gather loop: %v", start)

	iter := c.backend.GetTxnCounts(0)

	data := make(map[string]int)

	dates := make([]string, 0)
	values := make([]string, 0)

	sync := NewSync()
	sync.setInit(0)

	for iter.Next(&transaction) {
		sync.add(1)

		// Transaction is passed by value since each iteration unmarshals a new tx into "transaction"

		go func(tx models.Transaction, sync Sync) {
			sync.recieve()

			stamp := time.Unix(int64(tx.Timestamp), 0).Format("2/01/06")

			data[stamp] += 1

			sync.send(0)
			sync.done()

		}(transaction, sync)

		sync.swapChannels()
		sync.wait(10)
	}

	sync.close(0)

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

	start := time.Now()
	log.Debugf("Start block gather loop: %v", start)

	iter := c.backend.GetBlocks(0)

	data := &chartdata{}
	data.init()

	sync := NewSync()
	sync.setInit(0)

	for iter.Next(&block) {
		sync.add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(b models.Block, sync Sync, data *chartdata) {
			prevStamp := sync.recieve()

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

			sync.send(b.Timestamp)
			sync.done()

		}(block, sync, data)

		sync.swapChannels()
		sync.wait(10)
	}

	// Wait for loop to end
	sync.close(block.Timestamp)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

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

type btime struct {
	block     int
	stamp     string
	prevstamp uint64
}

func (u *btime) rotate(ps uint64) {
	u.block--
	u.prevstamp = ps
}

func (u *btime) set(ns string) {
	u.stamp = ns
}

func (c *Crawler) ChartBlocktime() {
	var block models.Block
	var wg sync.WaitGroup
	var routines int
	var c1, c2 chan *btime

	start := time.Now()
	log.Debugf("Start blocktime gather loop: %v", start)

	iter := c.backend.GetBlocks(365)

	data := make(map[string]*big.Int)

	// goroutine syncing patter from block crawler

	c2 = make(chan *btime, 1)

	dates := make([]string, 0)
	blocktime := make([]string, 0)

	c2 <- &btime{block: 88, stamp: "", prevstamp: 0}

	c1, c2 = c2, make(chan *btime, 1)

	for iter.Next(&block) {
		wg.Add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(wg *sync.WaitGroup, b models.Block, c1 chan *btime, c2 chan *btime) {
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
				u = &btime{
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

		c1, c2 = c2, make(chan *btime, 1)

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

		log.Debugf("End blocktime loop: %v", time.Since(start))
	}
}

type mined_blocks struct {
	Miners map[string]int64
}

func (p *mined_blocks) Add(nph interface{}) {
	for k, v := range nph.(*mined_blocks).Miners {
		if p.Miners[k] == 0 {
			p.Miners[k] = v
		} else {
			p.Miners[k] += v
		}
	}
}

func (p *mined_blocks) Map() map[string]string {
	result := make(map[string]string)
	var acc int64

	for k, v := range p.Miners {
		acc += v
		r := strconv.FormatInt(v, 10)
		if r == "" {
			r = "0"
		}
		result[k] = r
	}

	result["total"] = strconv.FormatInt(acc, 10)

	return result
}

func (c *Crawler) ChartMinedBlocks() {
	var block models.Block

	start := time.Now()
	log.Debugf("Start hashrate gather loop: %v", start)

	iter := c.backend.GetBlocks(0)

	data := &chartdata{}
	data.init()

	// goroutine syncing patter from block crawler

	sync := NewSync()
	sync.setInit(0)

	for iter.Next(&block) {
		sync.add(1)

		// Block is passed by value since each iteration unmarshals a new blocks into "block"

		go func(b models.Block, sync Sync, data *chartdata) {
			sync.recieve()

			stamp := time.Unix(int64(b.Timestamp), 0).Format("2/01/06")

			em := &mined_blocks{
				Miners: map[string]int64{
					b.Miner: 1,
				},
			}

			data.addElement(stamp, em)

			sync.send(b.Timestamp)
			sync.done()
		}(block, sync, data)

		sync.swapChannels()
		sync.wait(10)
	}

	// Wait for loop to end
	sync.close(block.Timestamp)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {

		dates := data.getDates()

		// hashrate := make([]map[string]string, 0)
		blocks := make(map[string][]string)

		for day, stamp := range dates {
			// TODO: Add some logic to split all the miners into slices of values

			// Divide each for no. of blocks
			elementMap := data.getElement(stamp).(*mined_blocks).Map()

			for k, b := range elementMap {
				if blocks[k] == nil {
					blocks[k] = make([]string, len(dates))
				}
				blocks[k][day] = b
			}

		}

		hashrateChart := &models.MLineChart{
			Chart:  "minedblocks",
			Labels: dates,
			Values: blocks,
		}

		c.backend.AddMLChart(hashrateChart)

		log.Debugf("End hashrate loop: %v", time.Since(start))
	}
}

func (c *Crawler) StoreUbqSupply() {
	var block models.Block

	start := time.Now()
	log.Debugf("Start ubq supply gather loop: %v", start)

	store, err := c.backend.SupplyObject("ubq")

	iter := c.backend.BlocksIter(store.LatestBlock.Number)

	s, _ := big.NewInt(0).SetString(store.Supply, 10)

	if err != nil {
		log.Errorf("Error getting supply: %v", err)
	}

	// goroutine syncing patter from block crawler

	sync := NewSync()
	sync.setInit(0)

	for iter.Next(&block) {
		sync.add(1)

		go func(b models.Block, sync Sync) {
			sync.recieve()

			minted, _ := new(big.Int).SetString(b.BlockReward, 10)
			s.Add(s, minted)

			sync.send(0)
			sync.done()
		}(block, sync)

		sync.swapChannels()
		sync.wait(10)
	}

	// Wait for loop to end
	sync.close(0)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {

		new_supply := &models.Store{
			Symbol:      "ubq",
			Timestamp:   time.Now().Unix(),
			Supply:      s.String(),
			LatestBlock: block,
			Price:       c.price,
		}

		c.backend.UpdateSupply("ubq", new_supply)

		log.Debugf("End ubq supply loop: %v", time.Since(start))
	}
}

func (c *Crawler) StoreQwarkSupply() {
	var tokentx models.TokenTransfer

	start := time.Now()
	log.Debugf("Start qwark supply gather loop: %v", start)

	store, err := c.backend.SupplyObject("qwark")

	// 0x4b4899a10f3e507db207b0ee2426029efa168a67 -- Qwark token address
	// 0xae3f04584446aa081cd98011f80f19977f8c10e0 -- Infinitum flame

	iter := c.backend.GetTokenTransfers("0x4b4899a10f3e507db207b0ee2426029efa168a67", "0xae3f04584446aa081cd98011f80f19977f8c10e0", store.Timestamp)

	if err != nil {
		log.Errorf("Error retrieving store/supply: %v", err)
	}

	s, _ := new(big.Int).SetString(store.Supply, 10)

	// goroutine syncing patter from block crawler

	sync := NewSync()
	sync.setInit(0)

	for iter.Next(&tokentx) {
		sync.add(1)

		go func(t models.TokenTransfer, sync Sync) {
			sync.recieve()

			minted, _ := new(big.Int).SetString(t.Value, 10)
			s.Add(s, minted)

			sync.send(0)
			sync.done()
		}(tokentx, sync)

		sync.swapChannels()
		sync.wait(10)
	}

	// Wait for loop to end
	sync.close(0)

	if err := iter.Err(); err != nil {
		log.Errorf("Error during iteration: %v", err)
	}

	if iter.Done() {

		new_supply := &models.Store{
			Symbol:    "qwark",
			Timestamp: time.Now().Unix(),
			Supply:    s.String(),
		}

		c.backend.UpdateSupply("qwark", new_supply)

		log.Debugf("End qwark supply loop: %v", time.Since(start))
	}
}
