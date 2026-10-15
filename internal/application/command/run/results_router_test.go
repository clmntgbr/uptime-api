package run

import (
	"context"
	"errors"
	"testing"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"
)

type stubPayloadHandler struct {
	calls int
	last  []byte
	err   error
}

func (h *stubPayloadHandler) Handle(_ context.Context, payload []byte) error {
	h.calls++
	h.last = append([]byte(nil), payload...)
	return h.err
}

func TestResultsRouter_DispatchesByType(t *testing.T) {
	result := &stubPayloadHandler{}
	progress := &stubPayloadHandler{}
	r := &ResultsRouter{result: result, progress: progress}

	if err := r.Handle(context.Background(), []byte(`{"type":"`+contract.CallProgressType+`"}`)); err != nil {
		t.Fatal(err)
	}
	if progress.calls != 1 || result.calls != 0 {
		t.Fatalf("progress=%d result=%d", progress.calls, result.calls)
	}

	if err := r.Handle(context.Background(), []byte(`{"type":"`+contract.ResultType+`"}`)); err != nil {
		t.Fatal(err)
	}
	if result.calls != 1 {
		t.Fatalf("result calls=%d", result.calls)
	}

	if err := r.Handle(context.Background(), []byte(`{"type":""}`)); err != nil {
		t.Fatal(err)
	}
	if result.calls != 2 {
		t.Fatalf("empty type should go to result, got %d", result.calls)
	}

	err := r.Handle(context.Background(), []byte(`{"type":"nope"}`))
	var nonRetryable *messaging.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected non-retryable, got %v", err)
	}
}
