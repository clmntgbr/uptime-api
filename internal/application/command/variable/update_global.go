package variable

import (
	"context"

	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type UpdateGlobalCommand struct {
	UserID uuid.UUID
	ID     uuid.UUID
	Value  string
	Secret bool
}

type UpdateGlobalHandler struct {
	repo variable.GlobalRepository
}

func NewUpdateGlobalHandler(repo variable.GlobalRepository) *UpdateGlobalHandler {
	return &UpdateGlobalHandler{repo: repo}
}

func (h *UpdateGlobalHandler) Handle(ctx context.Context, cmd UpdateGlobalCommand) (*variable.Global, error) {
	var saved *variable.Global
	err := h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		existing, err := h.repo.FindByID(txCtx, cmd.UserID, cmd.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return variable.ErrNotFound
		}
		if err := existing.Change(cmd.Value, cmd.Secret); err != nil {
			return err
		}
		saved = existing
		return h.repo.Save(txCtx, existing)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}
