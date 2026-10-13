package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type SetGlobalVariableCommand struct {
	UserID uuid.UUID
	Name   string
	Value  string
	Secret bool
}

type SetGlobalVariableHandler struct {
	repo      variable.GlobalRepository
	workflows domainworkflow.WriteRepository
}

func NewSetGlobalVariableHandler(repo variable.GlobalRepository, workflows domainworkflow.WriteRepository) *SetGlobalVariableHandler {
	return &SetGlobalVariableHandler{repo: repo, workflows: workflows}
}

// Handle creates or replaces a global. A new name must not already be a
// static or call variable of one of the user's workflows: names share one namespace.
func (h *SetGlobalVariableHandler) Handle(ctx context.Context, cmd SetGlobalVariableCommand) (*variable.Global, error) {
	var saved *variable.Global
	err := h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		existing, err := h.repo.FindByName(txCtx, cmd.UserID, cmd.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			if err := existing.Change(cmd.Value, cmd.Secret); err != nil {
				return err
			}
			saved = existing
			return h.repo.Save(txCtx, existing)
		}

		g, err := variable.NewGlobal(cmd.UserID, cmd.Name, cmd.Value, cmd.Secret)
		if err != nil {
			return err
		}
		workflows, err := h.workflows.ListByUser(txCtx, cmd.UserID)
		if err != nil {
			return err
		}
		for _, w := range workflows {
			for _, local := range w.LocalVariableNames() {
				if local == g.Name {
					return &domainworkflow.ValidationError{
						Variable: g.Name,
						Message:  fmt.Sprintf("name already used by workflow %q", w.Name),
						Kind:     domainworkflow.ErrNameConflict,
					}
				}
			}
		}
		saved = g
		return h.repo.Save(txCtx, g)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}
