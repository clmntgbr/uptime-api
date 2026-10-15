package run

import (
	"context"
	"encoding/json"
	"fmt"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"
)

type resultsPayloadHandler interface {
	Handle(ctx context.Context, payload []byte) error
}

// ResultsRouter dispatches workflow.results payloads by message type.
type ResultsRouter struct {
	result   resultsPayloadHandler
	progress resultsPayloadHandler
}

func NewResultsRouter(result *RecordResultHandler, progress *RecordCallProgressHandler) *ResultsRouter {
	return &ResultsRouter{result: result, progress: progress}
}

func (r *ResultsRouter) Handle(ctx context.Context, payload []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &head); err != nil {
		return messaging.NonRetryable(fmt.Errorf("decode results envelope: %w", err))
	}
	switch head.Type {
	case contract.CallProgressType:
		return r.progress.Handle(ctx, payload)
	case contract.ResultType, "":
		// Empty type: legacy payloads before type was required — treat as final result.
		return r.result.Handle(ctx, payload)
	default:
		return messaging.NonRetryable(fmt.Errorf("unknown workflow.results type %q", head.Type))
	}
}
