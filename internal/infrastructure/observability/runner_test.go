package observability

import (
	"context"
	"testing"
	"time"

	"go-api/internal/contract"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type stubRunner struct{ result contract.Result }

func (s stubRunner) Run(context.Context, contract.Job, map[string]string) contract.Result {
	return s.result
}

func sampleCount(t *testing.T, vec *prometheus.HistogramVec, status string) uint64 {
	t.Helper()
	var m dto.Metric
	if err := vec.WithLabelValues(status).(prometheus.Metric).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}

func TestInstrumentRunner_ObservesRunAndExecutedCalls(t *testing.T) {
	start := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	result := contract.Result{
		Status:     contract.RunStatusFailed,
		StartedAt:  start,
		FinishedAt: start.Add(3 * time.Second),
		Calls: []contract.CallResult{
			{Status: contract.CallStatusSuccess, StartedAt: start, FinishedAt: start.Add(time.Second)},
			{Status: contract.CallStatusFailed, StartedAt: start.Add(time.Second), FinishedAt: start.Add(3 * time.Second)},
			{Status: contract.CallStatusSkipped},
		},
	}

	got := InstrumentRunner(stubRunner{result: result}).Run(context.Background(), contract.Job{}, nil)

	if got.Status != result.Status {
		t.Fatalf("result not passed through: %q", got.Status)
	}
	if n := sampleCount(t, runDuration, contract.RunStatusFailed); n != 1 {
		t.Fatalf("expected 1 failed run observation, got %d", n)
	}
	if n := sampleCount(t, callDuration, contract.CallStatusSuccess); n != 1 {
		t.Fatalf("expected 1 success call observation, got %d", n)
	}
	if n := sampleCount(t, callDuration, contract.CallStatusSkipped); n != 0 {
		t.Fatalf("skipped calls must not be observed, got %d", n)
	}
}
