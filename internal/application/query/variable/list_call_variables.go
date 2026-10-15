package variable

import (
	"context"

	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListCallVariablesQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type ListCallVariablesHandler struct {
	repo domainworkflow.WriteRepository
}

func NewListCallVariablesHandler(repo domainworkflow.WriteRepository) *ListCallVariablesHandler {
	return &ListCallVariablesHandler{repo: repo}
}

func (h *ListCallVariablesHandler) Handle(ctx context.Context, q ListCallVariablesQuery) ([]domaincall.Extraction, error) {
	w, err := loadOwnedWorkflow(ctx, h.repo, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	return w.ListCallVariables(q.CallID)
}
