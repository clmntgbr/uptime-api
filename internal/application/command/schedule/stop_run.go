package schedule

import (
	"context"
	"fmt"
	"log/slog"

	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// StopRunCommand asks the active run of a workflow to stop after the current call.
type StopRunCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type StopRunHandler struct {
	states    domainschedule.Repository
	workflows domainworkflow.WriteRepository
	publisher port.CancelPublisher
	clock     port.Clock
	logger    *slog.Logger
}

func NewStopRunHandler(
	states domainschedule.Repository,
	workflows domainworkflow.WriteRepository,
	publisher port.CancelPublisher,
	clock port.Clock,
	logger *slog.Logger,
) *StopRunHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StopRunHandler{
		states: states, workflows: workflows, publisher: publisher, clock: clock, logger: logger,
	}
}

// Handle sets status=cancelling and signals worker-calls to skip remaining calls.
// Returns ErrNoRunInProgress when nothing is running.
func (h *StopRunHandler) Handle(ctx context.Context, cmd StopRunCommand) (uuid.UUID, error) {
	now := h.clock.Now()
	var (
		runID      uuid.UUID
		userID     uuid.UUID
		workflowID uuid.UUID
	)

	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := h.workflows.GetByID(txCtx, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if w == nil || w.UserID != cmd.UserID {
			return domainworkflow.ErrNotFound
		}

		state, err := h.states.GetForUpdate(txCtx, w.ID)
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("schedule state missing for workflow %s", w.ID)
		}

		id, ok := state.RequestCancel(now)
		if !ok {
			return domainworkflow.ErrNoRunInProgress
		}
		if err := h.states.Save(txCtx, *state); err != nil {
			return err
		}
		runID = id
		userID = w.UserID
		workflowID = w.ID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	cancel := contract.Cancel{
		V:           contract.CancelVersion,
		ID:          uuid.NewString(),
		Type:        contract.CancelType,
		UserID:      userID.String(),
		WorkflowID:  workflowID.String(),
		RunID:       runID.String(),
		RequestedAt: now.UTC(),
	}
	if err := h.publisher.PublishCancel(ctx, cancel); err != nil {
		h.logger.Error("cancel publish failed", "workflow_id", workflowID, "run_id", runID, "error", err)
		return uuid.Nil, fmt.Errorf("publish cancel: %w", err)
	}
	return runID, nil
}
