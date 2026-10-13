package observability

import (
	"context"

	"go-api/internal/contract"
	"go-api/internal/domain/port"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	runDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "workflow_run_duration_seconds",
		Help:    "Duration of a workflow run, from the first call to the last one.",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
	}, []string{"status"})
	callDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "workflow_call_duration_seconds",
		Help:    "Duration of a single call of a workflow run.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"status"})
)

type instrumentedRunner struct {
	next port.WorkflowRunner
}

// InstrumentRunner records run and call durations of every executed job.
func InstrumentRunner(next port.WorkflowRunner) port.WorkflowRunner {
	return instrumentedRunner{next: next}
}

func (r instrumentedRunner) Run(ctx context.Context, job contract.Job, secrets map[string]string) contract.Result {
	result := r.next.Run(ctx, job, secrets)
	runDuration.WithLabelValues(result.Status).Observe(result.FinishedAt.Sub(result.StartedAt).Seconds())
	for _, call := range result.Calls {
		if call.Status == contract.CallStatusSkipped {
			continue
		}
		callDuration.WithLabelValues(call.Status).Observe(call.FinishedAt.Sub(call.StartedAt).Seconds())
	}
	return result
}
