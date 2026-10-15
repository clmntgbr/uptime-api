package variable

import (
	"context"

	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetCallVariableQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
	Name       string
}

type GetCallVariableHandler struct {
	repo domainworkflow.WriteRepository
}

func NewGetCallVariableHandler(repo domainworkflow.WriteRepository) *GetCallVariableHandler {
	return &GetCallVariableHandler{repo: repo}
}

func (h *GetCallVariableHandler) Handle(ctx context.Context, q GetCallVariableQuery) (*domaincall.Extraction, error) {
	w, err := loadOwnedWorkflow(ctx, h.repo, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	return w.FindCallVariable(q.CallID, q.Name)
}
