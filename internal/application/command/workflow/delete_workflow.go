package workflow

import (
	"context"

	"go-api/internal/domain/port"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type DeleteWorkflowCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type DeleteWorkflowHandler struct {
	repo   domainworkflow.WriteRepository
	outbox port.OutboxRepository
}

func NewDeleteWorkflowHandler(repo domainworkflow.WriteRepository, outbox port.OutboxRepository) *DeleteWorkflowHandler {
	return &DeleteWorkflowHandler{repo: repo, outbox: outbox}
}

// Handle deletes the configuration; past runs stay until the retention purge.
func (h *DeleteWorkflowHandler) Handle(ctx context.Context, cmd DeleteWorkflowCommand) error {
	return h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
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
		w.MarkDeleted()
		if err := h.repo.Delete(txCtx, w.ID); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
}
