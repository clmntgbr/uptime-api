package contextpanel

import (
	"context"

	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetLatestRunQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type GetLatestRunHandler struct {
	workflows domainworkflow.ReadRepository
	runs      domainrun.ReadRepository
}

func NewGetLatestRunHandler(
	workflows domainworkflow.ReadRepository,
	runs domainrun.ReadRepository,
) *GetLatestRunHandler {
	return &GetLatestRunHandler{workflows: workflows, runs: runs}
}

// Handle returns the latest terminal run, or nil when the workflow has none.
// ErrNotFound when the workflow is missing or unowned.
func (h *GetLatestRunHandler) Handle(ctx context.Context, q GetLatestRunQuery) (*domainrun.LatestView, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, domainworkflow.ErrNotFound
	}
	return h.runs.FindLatestTerminalByWorkflow(ctx, q.UserID, q.WorkflowID)
}
