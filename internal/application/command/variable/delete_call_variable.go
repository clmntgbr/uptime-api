package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type DeleteCallVariableCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
	Name       string
}

type DeleteCallVariableHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewDeleteCallVariableHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *DeleteCallVariableHandler {
	return &DeleteCallVariableHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *DeleteCallVariableHandler) Handle(ctx context.Context, cmd DeleteCallVariableCommand) error {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return fmt.Errorf("load global variables: %w", err)
	}
	return h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if err := w.RemoveCallVariable(cmd.CallID, cmd.Name, globals); err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
}
