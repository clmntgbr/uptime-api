package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type CreateGlobalCommand struct {
	UserID uuid.UUID
	Name   string
	Value  string
	Secret bool
}

type CreateGlobalHandler struct {
	repo      variable.GlobalRepository
	workflows domainworkflow.WriteRepository
}

func NewCreateGlobalHandler(repo variable.GlobalRepository, workflows domainworkflow.WriteRepository) *CreateGlobalHandler {
	return &CreateGlobalHandler{repo: repo, workflows: workflows}
}

func (h *CreateGlobalHandler) Handle(ctx context.Context, cmd CreateGlobalCommand) (*variable.Global, error) {
	var saved *variable.Global
	err := h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		existing, err := h.repo.FindByName(txCtx, cmd.UserID, cmd.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			return variable.ErrAlreadyExists
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
