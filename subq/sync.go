package subq

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Sync struct {
	synctype string
	routines int
	c1, c2   chan uint64
	logChan  chan *logObject
	wg       *sync.WaitGroup
}

func (s *Sync) close(current uint64) {
closer:
	for {
		select {
		case close := <-s.c1:
			if close == current {
				break closer
			}
		}
	}
	close(s.c1)
	close(s.c2)
	close(s.logChan)

}

func (s *Sync) log(blockNo uint64, txns, tokentransfers, uncleNo int) {
	s.logChan <- &logObject{
		blockNo:        blockNo,
		txns:           txns,
		tokentransfers: tokentransfers,
		uncleNo:        uncleNo,
	}
}

func (s *Sync) swapChannels() {
	s.c1, s.c2 = s.c2, make(chan uint64, 1)
}

func (s *Sync) setInit(n uint64) {
	s.c2 <- n
	s.swapChannels()
}

func (s *Sync) setType(t string) {
	log.Info("synctype: ", t)
	s.synctype = t
}

func (s *Sync) recieve() uint64 {
	return <-s.c1
}

func (s *Sync) send(n uint64) {
	s.c2 <- n
}

func (s *Sync) add(n int) {
	s.routines += n
	s.wg.Add(n)
}

func (s *Sync) done() {
	s.wg.Done()
}

func (s *Sync) wait(max int) {
	if s.routines == max {
		s.wg.Wait()
		s.routines = 0
	}
}

func NewSync() Sync {

	wg := new(sync.WaitGroup)

	logchan := make(chan *logObject)

	// Start logging goroutine

	go func(ch chan *logObject) {
		start := time.Now()
		stats := &logObject{
			0,
			0,
			0,
			0,
			0,
		}
	logloop:
		for {
			select {
			case lo, ok := <-ch:
				if !ok {
					if stats.blocks > 0 {
						log.Printf("Added %v block(s) (head: %v)   \twith     \t%v transactions   \t%v tokentransfers   \t%v uncles\ttook %v", stats.blocks, stats.blockNo, stats.txns, stats.tokentransfers, stats.uncleNo, time.Since(start))
					}
					break logloop
				}
				stats.add(lo)

				if stats.blocks >= 1000 || time.Now().After(start.Add(time.Minute)) {
					log.Printf("Added %v blocks (head: %v)   \twith     \t%v transactions   \t%v tokentransfers   \t%v uncles\ttook %v", stats.blocks, stats.blockNo, stats.txns, stats.tokentransfers, stats.uncleNo, time.Since(start))
					stats.clear()
					start = time.Now()
				}
			}
		}
	}(logchan)

	sync := Sync{
		c1:      make(chan uint64, 1),
		c2:      make(chan uint64, 1),
		wg:      wg,
		logChan: logchan,
	}

	return sync
}
