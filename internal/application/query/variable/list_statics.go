package variable

import (
	"context"

	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListStaticsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type ListStaticsHandler struct {
	repo domainworkflow.WriteRepository
}

func NewListStaticsHandler(repo domainworkflow.WriteRepository) *ListStaticsHandler {
	return &ListStaticsHandler{repo: repo}
}

func (h *ListStaticsHandler) Handle(ctx context.Context, q ListStaticsQuery) ([]domainworkflow.StaticVariable, error) {
	w, err := loadOwnedWorkflow(ctx, h.repo, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	out := make([]domainworkflow.StaticVariable, len(w.Statics))
	copy(out, w.Statics)
	return out, nil
}
