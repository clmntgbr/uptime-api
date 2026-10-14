package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// RecordRunHandlerName dedups run recording: the worker and the scheduler (timed
// out runs) both mark (run_id, record_run), so a run is recorded once.
const RecordRunHandlerName = "record_run"

type DispatchConfig struct {
	BatchSize   int
	LeaseMargin time.Duration
	MaxJitter   time.Duration
}

type DispatchStats struct {
	Dispatched    int
	Skipped       int
	TimedOut      int
	PublishFailed int
	PrepareFailed int
	// Lag is how late the oldest due workflow was.
	Lag time.Duration
}

type DispatchDueRunsHandler struct {
	states    domainschedule.Repository
	workflows domainworkflow.WriteRepository
	globals   variable.GlobalRepository
	runs      domainrun.WriteRepository
	processed port.ProcessedEventRepository
	outbox    port.OutboxRepository
	sealer    port.JobSealer
	publisher port.JobPublisher
	clock     port.Clock
	cfg       DispatchConfig
	logger    *slog.Logger
}

func NewDispatchDueRunsHandler(
	states domainschedule.Repository,
	workflows domainworkflow.WriteRepository,
	globals variable.GlobalRepository,
	runs domainrun.WriteRepository,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
	sealer port.JobSealer,
	publisher port.JobPublisher,
	clock port.Clock,
	cfg DispatchConfig,
	logger *slog.Logger,
) *DispatchDueRunsHandler {
	return &DispatchDueRunsHandler{
		states: states, workflows: workflows, globals: globals, runs: runs, processed: processed,
		outbox: outbox, sealer: sealer, publisher: publisher, clock: clock, cfg: cfg, logger: logger,
	}
}

type pendingJob struct {
	job         contract.Job
	jitter      time.Duration
	scheduledAt time.Time
	oneShot     bool
}

// Handle leases every due workflow in one transaction, then publishes the jobs
// after commit, spread over MaxJitter. A job is never published for a lease
// that was not committed.
func (h *DispatchDueRunsHandler) Handle(ctx context.Context) (DispatchStats, error) {
	var (
		stats   DispatchStats
		pending []pendingJob
	)
	now := h.clock.Now()

	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		due, err := h.states.LockDue(txCtx, now, h.cfg.BatchSize)
		if err != nil {
			return fmt.Errorf("lock due workflows: %w", err)
		}
		globalsByUser := map[uuid.UUID][]variable.Global{}

		for _, d := range due {
			state := d.State
			if !state.NextRunAt.Equal(domainschedule.Never) {
				if lag := now.Sub(state.NextRunAt); lag > stats.Lag {
					stats.Lag = lag
				}
			}
			scheduledAt := state.NextRunAt
			dueToRun := d.Enabled && !state.NextRunAt.After(now)

			if state.LeaseActive(now) {
				// Interval moves on; one-shot waits until the lease ends to avoid skip spam.
				if isOneShot(d.ScheduleType) {
					if state.LeaseUntil != nil {
						state.NextRunAt = state.LeaseUntil.UTC()
					}
				} else {
					state.NextRunAt = domainschedule.NextRun(now, d.Interval)
				}
				if err := h.record(txCtx, domainrun.NewSkipped(state.WorkflowID, d.UserID, now)); err != nil {
					return err
				}
				stats.Skipped++
				if err := h.states.Save(txCtx, state); err != nil {
					return err
				}
				continue
			}

			if lease, expired := state.ExpireLease(); expired {
				if err := h.recordTimeout(txCtx, lease, state.WorkflowID, d.UserID); err != nil {
					return err
				}
				stats.TimedOut++
				// A lost result on a parked/disabled one-shot must still finish it.
				if isOneShot(d.ScheduleType) && (state.NextRunAt.Equal(domainschedule.Never) || !d.Enabled) {
					if err := h.disableOneShot(txCtx, state.WorkflowID); err != nil {
						return err
					}
					state.ClearSchedule()
					if err := h.states.Save(txCtx, state); err != nil {
						return err
					}
					continue
				}
			}

			if !dueToRun {
				if err := h.states.Save(txCtx, state); err != nil {
					return err
				}
				continue
			}

			w, err := h.workflows.GetByID(txCtx, state.WorkflowID)
			if err != nil {
				return fmt.Errorf("load workflow %s: %w", state.WorkflowID, err)
			}
			if w == nil {
				continue
			}
			jitter := h.jitter()
			runID := uuid.New()
			job, err := h.prepare(txCtx, w, globalsByUser, runID, scheduledAt, now.Add(jitter+w.Timeout), now)
			if err != nil {
				// One workflow that cannot be prepared (undecryptable secret)
				// must not block the others: it waits for its next pass.
				h.logger.Error("job preparation failed", "workflow_id", w.ID, "error", err)
				stats.PrepareFailed++
				if !isOneShot(d.ScheduleType) {
					state.NextRunAt = domainschedule.NextRun(now, d.Interval)
				}
				if err := h.states.Save(txCtx, state); err != nil {
					return err
				}
				continue
			}

			oneShot := isOneShot(d.ScheduleType)
			advanceSchedule(&state, d, now)
			state.Lease(runID, now, jitter+w.Timeout+h.cfg.LeaseMargin)
			if err := h.states.Save(txCtx, state); err != nil {
				return err
			}
			pending = append(pending, pendingJob{
				job: job, jitter: jitter, scheduledAt: scheduledAt, oneShot: oneShot,
			})
		}
		return nil
	})
	if err != nil {
		return stats, err
	}

	stats.Dispatched, stats.PublishFailed = h.publish(ctx, pending)
	return stats, nil
}

func advanceSchedule(state *domainschedule.State, d domainschedule.Due, now time.Time) {
	if isOneShot(d.ScheduleType) {
		state.ClearSchedule()
		return
	}
	state.NextRunAt = domainschedule.NextRun(now, d.Interval)
}

func isOneShot(scheduleType string) bool {
	return scheduleType == string(domainworkflow.ScheduleOnce) || scheduleType == string(domainworkflow.ScheduleAt)
}

func (h *DispatchDueRunsHandler) disableOneShot(ctx context.Context, workflowID uuid.UUID) error {
	w, err := h.workflows.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}
	if w == nil || !w.ScheduleType.IsOneShot() || !w.Disable() {
		return nil
	}
	if err := h.workflows.Update(ctx, w); err != nil {
		return err
	}
	return h.outbox.StoreEvents(ctx, w.PullEvents())
}

func (h *DispatchDueRunsHandler) prepare(
	ctx context.Context,
	w *domainworkflow.Workflow,
	globalsByUser map[uuid.UUID][]variable.Global,
	runID uuid.UUID,
	scheduledAt, deadline, now time.Time,
) (contract.Job, error) {
	globals, ok := globalsByUser[w.UserID]
	if !ok {
		var err error
		if globals, err = h.globals.ListByUser(ctx, w.UserID); err != nil {
			return contract.Job{}, fmt.Errorf("load globals: %w", err)
		}
		globalsByUser[w.UserID] = globals
	}
	return prepareJob(h.sealer, w, globals, runID, contract.TriggerSchedule, scheduledAt, deadline, now)
}

func (h *DispatchDueRunsHandler) record(ctx context.Context, r *domainrun.Run) error {
	if err := h.runs.Save(ctx, r); err != nil {
		return fmt.Errorf("record run: %w", err)
	}
	return h.outbox.StoreEvents(ctx, r.PullEvents())
}

func (h *DispatchDueRunsHandler) recordTimeout(ctx context.Context, lease domainschedule.ExpiredLease, workflowID, userID uuid.UUID) error {
	claimed, err := h.processed.MarkProcessed(ctx, lease.RunID.String(), RecordRunHandlerName)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	return h.record(ctx, domainrun.NewTimedOut(lease.RunID, workflowID, userID, lease.StartedAt, lease.Until))
}

func (h *DispatchDueRunsHandler) jitter() time.Duration {
	if h.cfg.MaxJitter <= 0 {
		return 0
	}
	return rand.N(h.cfg.MaxJitter)
}

func (h *DispatchDueRunsHandler) publish(ctx context.Context, pending []pendingJob) (int, int) {
	var (
		wg               sync.WaitGroup
		mu               sync.Mutex
		published, fails int
	)
	for _, p := range pending {
		wg.Add(1)
		go func(p pendingJob) {
			defer wg.Done()
			select {
			case <-time.After(p.jitter):
			case <-ctx.Done():
			}
			err := h.publisher.PublishJob(context.WithoutCancel(ctx), p.job)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				published++
				return
			}
			fails++
			h.logger.Error("job publish failed", "workflow_id", p.job.WorkflowID, "run_id", p.job.RunID, "error", err)
			h.release(context.WithoutCancel(ctx), p)
		}(p)
	}
	wg.Wait()
	return published, fails
}

// release frees the lease of a job that could not be published, so the
// workflow runs again at its next pass instead of waiting for the lease to expire.
func (h *DispatchDueRunsHandler) release(ctx context.Context, p pendingJob) {
	workflowID, _ := uuid.Parse(p.job.WorkflowID)
	runID, _ := uuid.Parse(p.job.RunID)
	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		state, err := h.states.GetForUpdate(txCtx, workflowID)
		if err != nil || state == nil {
			return err
		}
		if !state.ReleaseLease(runID) {
			return nil
		}
		if p.oneShot {
			state.NextRunAt = p.scheduledAt
		}
		return h.states.Save(txCtx, *state)
	})
	if err != nil {
		h.logger.Error("lease release failed", "workflow_id", p.job.WorkflowID, "run_id", p.job.RunID, "error", err)
	}
}
