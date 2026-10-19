package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"
	"go-api/internal/engine"
)

// RecordCancelHandler marks a run as cancelled in the in-memory registry so the
// engine skips remaining calls after the current one finishes.
type RecordCancelHandler struct {
	registry *engine.MemoryCancellationRegistry
}

func NewRecordCancelHandler(registry *engine.MemoryCancellationRegistry) *RecordCancelHandler {
	return &RecordCancelHandler{registry: registry}
}

func (h *RecordCancelHandler) Handle(_ context.Context, payload []byte) error {
	var cancel contract.Cancel
	if err := json.Unmarshal(payload, &cancel); err != nil {
		return messaging.NonRetryable(fmt.Errorf("decode cancel: %w", err))
	}
	if err := cancel.Validate(); err != nil {
		return messaging.NonRetryable(err)
	}
	h.registry.Mark(cancel.RunID)
	return nil
}
