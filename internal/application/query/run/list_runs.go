package run

import (
	"context"

	"go-api/internal/domain/paginate"
	domainrun "go-api/internal/domain/run"

	"github.com/google/uuid"
)

type ListRunsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Query      paginate.PaginateQuery
}

type ListRunsHandler struct {
	repo domainrun.ReadRepository
}

func NewListRunsHandler(repo domainrun.ReadRepository) *ListRunsHandler {
	return &ListRunsHandler{repo: repo}
}

func (h *ListRunsHandler) Handle(ctx context.Context, q ListRunsQuery) ([]domainrun.View, int64, error) {
	return h.repo.ListByWorkflow(ctx, q.UserID, q.WorkflowID, q.Query)
}
