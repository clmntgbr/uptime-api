package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type DeleteStaticCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Name       string
}

type DeleteStaticHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewDeleteStaticHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *DeleteStaticHandler {
	return &DeleteStaticHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *DeleteStaticHandler) Handle(ctx context.Context, cmd DeleteStaticCommand) error {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return fmt.Errorf("load global variables: %w", err)
	}
	return h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if err := w.RemoveStatic(cmd.Name, globals); err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
}
