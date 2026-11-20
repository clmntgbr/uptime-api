package schedule

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"

	"github.com/google/uuid"
)

// recordTimedOutRun writes the timed-out run for a lease that never reported
// back. processed_events claims the run, so a second pass does nothing.
func recordTimedOutRun(
	ctx context.Context,
	processed port.ProcessedEventRepository,
	runs domainrun.WriteRepository,
	outbox port.OutboxRepository,
	lease domainschedule.ExpiredLease,
	workflowID, userID uuid.UUID,
) error {
	claimed, err := processed.MarkProcessed(ctx, lease.RunID.String(), RecordRunHandlerName)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	r := domainrun.NewTimedOut(lease.RunID, workflowID, userID, lease.StartedAt, lease.Until)
	if err := runs.Save(ctx, r); err != nil {
		return fmt.Errorf("record timed out run: %w", err)
	}
	return outbox.StoreEvents(ctx, r.PullEvents())
}

// releaseLease frees the lease held by runID. adjust, when set, may change the
// schedule in the same write (a one-shot job goes back to its due time).
// It reports whether the lease was actually held by runID.
func releaseLease(
	ctx context.Context,
	states domainschedule.Repository,
	workflowID, runID uuid.UUID,
	adjust func(*domainschedule.State),
) error {
	return states.WithTransaction(ctx, func(txCtx context.Context) error {
		state, err := states.GetForUpdate(txCtx, workflowID)
		if err != nil || state == nil {
			return err
		}
		if !state.ReleaseLease(runID) {
			return nil
		}
		if adjust != nil {
			adjust(state)
		}
		return states.Save(txCtx, *state)
	})
}
