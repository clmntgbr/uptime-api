package kafka

import (
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
)

// partitionState tracks the records of one assigned partition that were handed
// to handlers. The committable offset is the oldest record still in flight:
// a record finished out of order never moves the commit past an unfinished one.
type partitionState struct {
	mu            sync.Mutex
	inflight      map[int64]struct{}
	maxDispatched int64
	committed     int64
}

func newPartitionState() *partitionState {
	return &partitionState{inflight: make(map[int64]struct{}), maxDispatched: -1, committed: -1}
}

func (p *partitionState) dispatched(offset int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inflight[offset] = struct{}{}
	if offset > p.maxDispatched {
		p.maxDispatched = offset
	}
}

func (p *partitionState) done(offset int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inflight, offset)
}

// commitPoint returns the next offset to commit (one after the last record
// whose predecessors are all done) and whether it moved since the last commit.
func (p *partitionState) commitPoint() (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.maxDispatched < 0 {
		return 0, false
	}
	point := p.maxDispatched + 1
	for offset := range p.inflight {
		if offset < point {
			point = offset
		}
	}
	return point, point > p.committed
}

func (p *partitionState) markCommitted(offset int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if offset > p.committed {
		p.committed = offset
	}
}

type topicPartition struct {
	topic     string
	partition int32
}

// offsetTracker holds the state of every partition currently assigned.
// A revoked partition is dropped; late completions on its old state are ignored.
type offsetTracker struct {
	mu    sync.Mutex
	parts map[topicPartition]*partitionState
}

func newOffsetTracker() *offsetTracker {
	return &offsetTracker{parts: make(map[topicPartition]*partitionState)}
}

func (t *offsetTracker) state(topic string, partition int32) *partitionState {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := topicPartition{topic, partition}
	st, ok := t.parts[key]
	if !ok {
		st = newPartitionState()
		t.parts[key] = st
	}
	return st
}

// uncommitted returns the offsets to commit, optionally limited to some partitions.
func (t *offsetTracker) uncommitted(only map[string][]int32) map[string]map[int32]kgo.EpochOffset {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make(map[string]map[int32]kgo.EpochOffset)
	for key, st := range t.parts {
		if only != nil && !contains(only[key.topic], key.partition) {
			continue
		}
		point, moved := st.commitPoint()
		if !moved {
			continue
		}
		if out[key.topic] == nil {
			out[key.topic] = make(map[int32]kgo.EpochOffset)
		}
		out[key.topic][key.partition] = kgo.EpochOffset{Epoch: -1, Offset: point}
	}
	return out
}

func (t *offsetTracker) markCommitted(offsets map[string]map[int32]kgo.EpochOffset) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for topic, partitions := range offsets {
		for partition, eo := range partitions {
			if st, ok := t.parts[topicPartition{topic, partition}]; ok {
				st.markCommitted(eo.Offset)
			}
		}
	}
}

func (t *offsetTracker) drop(partitions map[string][]int32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for topic, ps := range partitions {
		for _, p := range ps {
			delete(t.parts, topicPartition{topic, p})
		}
	}
}

func contains(values []int32, v int32) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
