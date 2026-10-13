package workflow

import (
	"context"
	"fmt"
	"time"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpdateWorkflowCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Definition domainworkflow.Definition
}

type UpdateWorkflowHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
	clock     port.Clock
}

func NewUpdateWorkflowHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
	clock port.Clock,
) *UpdateWorkflowHandler {
	return &UpdateWorkflowHandler{repo: repo, variables: variables, outbox: outbox, clock: clock}
}

// Handle replaces the definition. A run in progress keeps the definition it was
// dispatched with; the new one applies from the next pass.
func (h *UpdateWorkflowHandler) Handle(ctx context.Context, cmd UpdateWorkflowCommand) (*domainworkflow.Workflow, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	var updated *domainworkflow.Workflow
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := h.repo.GetByID(txCtx, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if w == nil {
			return domainworkflow.ErrNotFound
		}
		if w.UserID != cmd.UserID {
			return domainworkflow.ErrWrongOwner
		}

		scheduleChanged, err := w.Update(cmd.Definition, globals)
		if err != nil {
			return err
		}
		var nextRunAt *time.Time
		if scheduleChanged {
			next := domainworkflow.NextRun(h.clock.Now(), w.Interval)
			nextRunAt = &next
		}
		if err := h.repo.Update(txCtx, w, nextRunAt); err != nil {
			return err
		}
		updated = w
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
