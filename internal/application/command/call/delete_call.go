package call

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type DeleteCallCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type DeleteCallHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewDeleteCallHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *DeleteCallHandler {
	return &DeleteCallHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *DeleteCallHandler) Handle(ctx context.Context, cmd DeleteCallCommand) error {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("load global variables: %w", err)
	}

	return h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if err := w.RemoveCall(cmd.CallID, globals); err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
}
