package call

import (
	"context"

	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetCallQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type GetCallHandler struct {
	repo domainworkflow.ReadRepository
}

func NewGetCallHandler(repo domainworkflow.ReadRepository) *GetCallHandler {
	return &GetCallHandler{repo: repo}
}

func (h *GetCallHandler) Handle(ctx context.Context, q GetCallQuery) (*domainworkflow.CallView, error) {
	workflow, err := h.repo.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, domainworkflow.ErrNotFound
	}
	view, err := h.repo.FindCall(ctx, q.UserID, q.WorkflowID, q.CallID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, domainworkflow.ErrCallNotFound
	}
	return view, nil
}
