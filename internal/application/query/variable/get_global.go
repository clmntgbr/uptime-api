package variable

import (
	"context"

	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type GetGlobalQuery struct {
	UserID uuid.UUID
	ID     uuid.UUID
}

type GetGlobalHandler struct {
	repo variable.GlobalRepository
}

func NewGetGlobalHandler(repo variable.GlobalRepository) *GetGlobalHandler {
	return &GetGlobalHandler{repo: repo}
}

func (h *GetGlobalHandler) Handle(ctx context.Context, q GetGlobalQuery) (*variable.Global, error) {
	g, err := h.repo.FindByID(ctx, q.UserID, q.ID)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, variable.ErrNotFound
	}
	return g, nil
}
