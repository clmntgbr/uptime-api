package variable

import (
	"context"
	"errors"
	"fmt"

	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

var ErrVariableInUse = errors.New("variable is used by a workflow")

type DeleteGlobalVariableCommand struct {
	UserID uuid.UUID
	ID     uuid.UUID
}

type DeleteGlobalVariableHandler struct {
	repo      variable.GlobalRepository
	workflows domainworkflow.WriteRepository
}

func NewDeleteGlobalVariableHandler(repo variable.GlobalRepository, workflows domainworkflow.WriteRepository) *DeleteGlobalVariableHandler {
	return &DeleteGlobalVariableHandler{repo: repo, workflows: workflows}
}

// Handle refuses to delete a global still referenced: the workflows using it
// would otherwise skip their calls on every run.
func (h *DeleteGlobalVariableHandler) Handle(ctx context.Context, cmd DeleteGlobalVariableCommand) error {
	return h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		existing, err := h.repo.FindByID(txCtx, cmd.UserID, cmd.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return variable.ErrNotFound
		}
		workflows, err := h.workflows.ListByUser(txCtx, cmd.UserID)
		if err != nil {
			return err
		}
		for _, w := range workflows {
			if w.UsesVariable(existing.Name) {
				return fmt.Errorf("%w: %q", ErrVariableInUse, w.Name)
			}
		}
		return h.repo.Delete(txCtx, cmd.UserID, cmd.ID)
	})
}
