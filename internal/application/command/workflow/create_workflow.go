package workflow

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type CreateWorkflowCommand struct {
	UserID     uuid.UUID
	Definition domainworkflow.Definition
}

type CreateWorkflowHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
	clock     port.Clock
}

func NewCreateWorkflowHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
	clock port.Clock,
) *CreateWorkflowHandler {
	return &CreateWorkflowHandler{repo: repo, variables: variables, outbox: outbox, clock: clock}
}

// Handle validates and stores the workflow; its first run is the next interval boundary.
func (h *CreateWorkflowHandler) Handle(ctx context.Context, cmd CreateWorkflowCommand) (*domainworkflow.Workflow, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	w, err := domainworkflow.New(cmd.UserID, cmd.Definition, globals)
	if err != nil {
		return nil, err
	}
	state := domainworkflow.NewState(w.ID, h.clock.Now(), w.Interval)

	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, w, state); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}
	return w, nil
}
