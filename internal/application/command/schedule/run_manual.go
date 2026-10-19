package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// RunManualCommand starts one run immediately. Concurrent runs are rejected.
type RunManualCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type RunManualHandler struct {
	states      domainschedule.Repository
	workflows   domainworkflow.WriteRepository
	globals     variable.GlobalRepository
	runs        domainrun.WriteRepository
	processed   port.ProcessedEventRepository
	outbox      port.OutboxRepository
	sealer      port.JobSealer
	publisher   port.JobPublisher
	clock       port.Clock
	leaseMargin time.Duration
	logger      *slog.Logger
}

func NewRunManualHandler(
	states domainschedule.Repository,
	workflows domainworkflow.WriteRepository,
	globals variable.GlobalRepository,
	runs domainrun.WriteRepository,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
	sealer port.JobSealer,
	publisher port.JobPublisher,
	clock port.Clock,
	leaseMargin time.Duration,
	logger *slog.Logger,
) *RunManualHandler {
	if leaseMargin <= 0 {
		leaseMargin = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RunManualHandler{
		states: states, workflows: workflows, globals: globals, runs: runs, processed: processed,
		outbox: outbox, sealer: sealer, publisher: publisher, clock: clock,
		leaseMargin: leaseMargin, logger: logger,
	}
}

// Handle leases the workflow and publishes a manual job. Returns ErrRunInProgress
// when another run still holds an active lease.
func (h *RunManualHandler) Handle(ctx context.Context, cmd RunManualCommand) (uuid.UUID, error) {
	now := h.clock.Now()
	var (
		job   contract.Job
		runID uuid.UUID
	)

	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := h.workflows.GetByID(txCtx, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if w == nil || w.UserID != cmd.UserID {
			return domainworkflow.ErrNotFound
		}
		if len(w.Calls) == 0 {
			return domainworkflow.ErrNoCalls
		}

		state, err := h.states.GetForUpdate(txCtx, w.ID)
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("schedule state missing for workflow %s", w.ID)
		}
		if state.LeaseActive(now) {
			// After stop, status stays cancelling until the current call finishes.
			// Allow an immediate re-run by taking over that lease; the previous
			// run keeps its cancel signal. Finalize it here so a late Result is a
			// no-op and cannot overwrite the new lease's outcome.
			old, ok := state.TakeOverCancelling(now)
			if !ok {
				return domainworkflow.ErrRunInProgress
			}
			if err := h.recordSuperseded(txCtx, old, state.WorkflowID, w.UserID, now); err != nil {
				return err
			}
		}
		if lease, expired := state.ExpireLease(); expired {
			if err := h.recordTimeout(txCtx, lease, state.WorkflowID, w.UserID); err != nil {
				return err
			}
		}

		runID = uuid.New()
		job, err = h.prepare(txCtx, w, runID, now)
		if err != nil {
			return err
		}
		state.Lease(runID, now, w.Timeout+h.leaseMargin)
		return h.states.Save(txCtx, *state)
	})
	if err != nil {
		return uuid.Nil, err
	}

	if err := h.publisher.PublishJob(ctx, job); err != nil {
		h.logger.Error("manual job publish failed", "workflow_id", job.WorkflowID, "run_id", job.RunID, "error", err)
		h.release(context.WithoutCancel(ctx), job)
		return uuid.Nil, fmt.Errorf("publish job: %w", err)
	}
	return runID, nil
}

func (h *RunManualHandler) prepare(ctx context.Context, w *domainworkflow.Workflow, runID uuid.UUID, now time.Time) (contract.Job, error) {
	globals, err := h.globals.ListByUser(ctx, w.UserID)
	if err != nil {
		return contract.Job{}, fmt.Errorf("load globals: %w", err)
	}
	return prepareJob(h.sealer, w, globals, runID, contract.TriggerManual, now, now.Add(w.Timeout), now)
}

func (h *RunManualHandler) recordTimeout(ctx context.Context, lease domainschedule.ExpiredLease, workflowID, userID uuid.UUID) error {
	claimed, err := h.processed.MarkProcessed(ctx, lease.RunID.String(), RecordRunHandlerName)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	r := domainrun.NewTimedOut(lease.RunID, workflowID, userID, lease.StartedAt, lease.Until)
	if err := h.runs.Save(ctx, r); err != nil {
		return fmt.Errorf("record timed out run: %w", err)
	}
	return h.outbox.StoreEvents(ctx, r.PullEvents())
}

// recordSuperseded finalizes a cancelling run that was replaced by a new manual
// run, so the late Result from worker-calls is ignored (processed_events).
func (h *RunManualHandler) recordSuperseded(ctx context.Context, lease domainschedule.ExpiredLease, workflowID, userID uuid.UUID, at time.Time) error {
	claimed, err := h.processed.MarkProcessed(ctx, lease.RunID.String(), RecordRunHandlerName)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	started := lease.StartedAt
	if started.IsZero() {
		started = at
	}
	r := domainrun.NewCancelled(lease.RunID, workflowID, userID, started, at)
	if err := h.runs.Save(ctx, r); err != nil {
		return fmt.Errorf("record superseded run: %w", err)
	}
	return h.outbox.StoreEvents(ctx, r.PullEvents())
}

func (h *RunManualHandler) release(ctx context.Context, job contract.Job) {
	workflowID, _ := uuid.Parse(job.WorkflowID)
	runID, _ := uuid.Parse(job.RunID)
	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		state, err := h.states.GetForUpdate(txCtx, workflowID)
		if err != nil || state == nil {
			return err
		}
		if !state.ReleaseLease(runID) {
			return nil
		}
		return h.states.Save(txCtx, *state)
	})
	if err != nil {
		h.logger.Error("manual lease release failed", "workflow_id", job.WorkflowID, "run_id", job.RunID, "error", err)
	}
}
