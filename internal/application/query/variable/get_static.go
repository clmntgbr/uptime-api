package variable

import (
	"context"

	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetStaticQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Name       string
}

type GetStaticHandler struct {
	repo domainworkflow.WriteRepository
}

func NewGetStaticHandler(repo domainworkflow.WriteRepository) *GetStaticHandler {
	return &GetStaticHandler{repo: repo}
}

func (h *GetStaticHandler) Handle(ctx context.Context, q GetStaticQuery) (*domainworkflow.StaticVariable, error) {
	w, err := loadOwnedWorkflow(ctx, h.repo, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	s := w.FindStatic(q.Name)
	if s == nil {
		return nil, domainworkflow.ErrVariableNotFound
	}
	return s, nil
}
