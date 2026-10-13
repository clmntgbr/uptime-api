package workflow

import (
	"context"

	"go-api/internal/domain/paginate"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListWorkflowsQuery struct {
	UserID uuid.UUID
	Query  paginate.PaginateQuery
}

type ListWorkflowsHandler struct {
	repo domainworkflow.ReadRepository
}

func NewListWorkflowsHandler(repo domainworkflow.ReadRepository) *ListWorkflowsHandler {
	return &ListWorkflowsHandler{repo: repo}
}

func (h *ListWorkflowsHandler) Handle(ctx context.Context, q ListWorkflowsQuery) ([]domainworkflow.ListView, int64, error) {
	return h.repo.List(ctx, q.UserID, q.Query)
}
