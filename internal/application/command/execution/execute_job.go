package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"
	"go-api/internal/domain/port"
)

const publishAttempts = 3

type ExecuteJobHandler struct {
	opener    port.JobOpener
	runner    port.WorkflowRunner
	publisher port.ResultPublisher
	clock     port.Clock
	logger    *slog.Logger
}

func NewExecuteJobHandler(opener port.JobOpener, runner port.WorkflowRunner, publisher port.ResultPublisher, clock port.Clock, logger *slog.Logger) *ExecuteJobHandler {
	return &ExecuteJobHandler{opener: opener, runner: runner, publisher: publisher, clock: clock, logger: logger}
}

// Handle runs one job from `workflow.jobs` and publishes its single result.
// It never touches a database. A job past its deadline is dropped: the
// scheduler has already recorded (or will record) the run as timed out.
func (h *ExecuteJobHandler) Handle(ctx context.Context, payload []byte) error {
	var job contract.Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return messaging.NonRetryable(fmt.Errorf("decode job: %w", err))
	}
	if err := job.Validate(); err != nil {
		return messaging.NonRetryable(err)
	}
	if h.clock.Now().After(job.Deadline) {
		h.logger.Warn("job dropped after its deadline", "run_id", job.RunID, "workflow_id", job.WorkflowID, "deadline", job.Deadline)
		return nil
	}

	secrets, err := h.opener.Open(job.Secrets)
	if err != nil {
		return messaging.NonRetryable(fmt.Errorf("open secrets of run %s: %w", job.RunID, err))
	}
	result := h.runner.Run(ctx, job, secrets)
	clear(secrets)

	// Publishing is retried here rather than by the consumer, which would run
	// every call of the workflow again.
	for attempt := 1; ; attempt++ {
		err = h.publisher.PublishResult(ctx, result)
		if err == nil {
			h.logger.Info("run executed", "run_id", job.RunID, "workflow_id", job.WorkflowID, "status", result.Status,
				"duration_ms", result.DurationMS, "success", result.Counts.Success, "failed", result.Counts.Failed,
				"error", result.Counts.Error, "skipped", result.Counts.Skipped)
			return nil
		}
		if attempt == publishAttempts || ctx.Err() != nil {
			return messaging.Retryable(fmt.Errorf("publish result of run %s: %w", job.RunID, err))
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
}
