package call

import (
	"context"

	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListCallsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type ListCallsHandler struct {
	repo domainworkflow.ReadRepository
}

func NewListCallsHandler(repo domainworkflow.ReadRepository) *ListCallsHandler {
	return &ListCallsHandler{repo: repo}
}

func (h *ListCallsHandler) Handle(ctx context.Context, q ListCallsQuery) ([]domainworkflow.CallListView, error) {
	workflow, err := h.repo.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, domainworkflow.ErrNotFound
	}
	return h.repo.ListCalls(ctx, q.UserID, q.WorkflowID)
}
