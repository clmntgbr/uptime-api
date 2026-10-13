package workflow

import (
	"context"

	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetWorkflowQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type GetWorkflowHandler struct {
	repo domainworkflow.ReadRepository
}

func NewGetWorkflowHandler(repo domainworkflow.ReadRepository) *GetWorkflowHandler {
	return &GetWorkflowHandler{repo: repo}
}

func (h *GetWorkflowHandler) Handle(ctx context.Context, q GetWorkflowQuery) (*domainworkflow.View, error) {
	view, err := h.repo.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, domainworkflow.ErrNotFound
	}
	return view, nil
}
