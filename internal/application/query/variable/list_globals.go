package variable

import (
	"context"

	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type ListGlobalsQuery struct {
	UserID uuid.UUID
}

type ListGlobalsHandler struct {
	repo variable.GlobalRepository
}

func NewListGlobalsHandler(repo variable.GlobalRepository) *ListGlobalsHandler {
	return &ListGlobalsHandler{repo: repo}
}

func (h *ListGlobalsHandler) Handle(ctx context.Context, q ListGlobalsQuery) ([]variable.Global, error) {
	return h.repo.ListByUser(ctx, q.UserID)
}
