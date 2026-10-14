package workflow

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/schedule"
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
	states    schedule.Repository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
	clock     port.Clock
}

func NewUpdateWorkflowHandler(
	repo domainworkflow.WriteRepository,
	states schedule.Repository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
	clock port.Clock,
) *UpdateWorkflowHandler {
	return &UpdateWorkflowHandler{repo: repo, states: states, variables: variables, outbox: outbox, clock: clock}
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
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		if scheduleChanged {
			if err := h.reschedule(txCtx, w); err != nil {
				return err
			}
		}
		updated = w
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (h *UpdateWorkflowHandler) reschedule(ctx context.Context, w *domainworkflow.Workflow) error {
	state, err := h.states.GetForUpdate(ctx, w.ID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("schedule state missing for workflow %s", w.ID)
	}
	if !w.Enabled {
		state.ClearSchedule()
		return h.states.Save(ctx, *state)
	}
	next, err := domainworkflow.NextRunAt(domainworkflow.Definition{
		ScheduleType: w.ScheduleType,
		Interval:     w.Interval,
		RunAt:        w.RunAt,
	}, h.clock.Now())
	if err != nil {
		return err
	}
	state.Reschedule(next)
	return h.states.Save(ctx, *state)
}
