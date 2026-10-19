package engine

import (
	"context"
	"sync"

	"go-api/internal/domain/port"
)

// MemoryCancellationRegistry tracks cancel signals for in-flight runs.
// worker-calls fills it from the workflow.cancels consumer.
type MemoryCancellationRegistry struct {
	mu  sync.RWMutex
	ids map[string]struct{}
}

func NewMemoryCancellationRegistry() *MemoryCancellationRegistry {
	return &MemoryCancellationRegistry{ids: make(map[string]struct{})}
}

func (r *MemoryCancellationRegistry) Mark(runID string) {
	if runID == "" {
		return
	}
	r.mu.Lock()
	r.ids[runID] = struct{}{}
	r.mu.Unlock()
}

func (r *MemoryCancellationRegistry) Clear(runID string) {
	r.mu.Lock()
	delete(r.ids, runID)
	r.mu.Unlock()
}

func (r *MemoryCancellationRegistry) IsCancelled(_ context.Context, runID string) (bool, error) {
	r.mu.RLock()
	_, ok := r.ids[runID]
	r.mu.RUnlock()
	return ok, nil
}

var _ port.CancellationChecker = (*MemoryCancellationRegistry)(nil)
