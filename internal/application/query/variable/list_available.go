package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListAvailableQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type ListAvailableHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
}

func NewListAvailableHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
) *ListAvailableHandler {
	return &ListAvailableHandler{repo: repo, variables: variables}
}

func (h *ListAvailableHandler) Handle(ctx context.Context, q ListAvailableQuery) ([]variable.Available, error) {
	globals, err := h.variables.ListByUser(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	w, err := loadOwnedWorkflow(ctx, h.repo, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	return w.AvailableVariables(q.CallID, globals)
}
