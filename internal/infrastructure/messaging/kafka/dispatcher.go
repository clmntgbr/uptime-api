package kafka

import (
	"context"
	"hash/fnv"
	"sync"
)

// dispatcher runs handlers concurrently. With ordered=true, records sharing a key
// go through the same lane and keep their partition order (results of one
// workflow are never applied concurrently); otherwise any free slot is used.
type dispatcher struct {
	ordered bool
	sem     chan struct{}
	lanes   []chan func()
	wg      sync.WaitGroup
	lanesWG sync.WaitGroup
}

func newDispatcher(concurrency int, ordered bool) *dispatcher {
	if concurrency <= 0 {
		concurrency = 1
	}
	d := &dispatcher{ordered: ordered}
	if !ordered {
		d.sem = make(chan struct{}, concurrency)
		return d
	}
	d.lanes = make([]chan func(), concurrency)
	for i := range d.lanes {
		lane := make(chan func(), 8)
		d.lanes[i] = lane
		d.lanesWG.Add(1)
		go func() {
			defer d.lanesWG.Done()
			for fn := range lane {
				fn()
				d.wg.Done()
			}
		}()
	}
	return d
}

// submit blocks while every slot (or the key's lane) is busy, which applies
// back-pressure on the poll loop. It returns false if ctx ended first.
func (d *dispatcher) submit(ctx context.Context, key []byte, fn func()) bool {
	d.wg.Add(1)

	if d.ordered {
		lane := d.lanes[laneIndex(key, len(d.lanes))]
		select {
		case lane <- fn:
			return true
		case <-ctx.Done():
			d.wg.Done()
			return false
		}
	}

	select {
	case d.sem <- struct{}{}:
	case <-ctx.Done():
		d.wg.Done()
		return false
	}
	go func() {
		defer func() {
			<-d.sem
			d.wg.Done()
		}()
		fn()
	}()
	return true
}

// close waits for every submitted handler, then stops the lanes.
func (d *dispatcher) close() {
	d.wg.Wait()
	for _, lane := range d.lanes {
		close(lane)
	}
	d.lanesWG.Wait()
}

func laneIndex(key []byte, n int) int {
	if n <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write(key)
	return int(h.Sum32() % uint32(n))
}
