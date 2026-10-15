package variable

import (
	"context"

	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListResponseKeysQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type ListResponseKeysHandler struct {
	workflows domainworkflow.ReadRepository
	runs      domainrun.ReadRepository
}

func NewListResponseKeysHandler(
	workflows domainworkflow.ReadRepository,
	runs domainrun.ReadRepository,
) *ListResponseKeysHandler {
	return &ListResponseKeysHandler{workflows: workflows, runs: runs}
}

// Handle returns JSON paths observed on the latest run of this call (for path
// autocomplete when declaring a call variable). Empty when the call never ran
// or no keys were collected.
func (h *ListResponseKeysHandler) Handle(ctx context.Context, q ListResponseKeysQuery) ([]string, error) {
	call, err := h.workflows.FindCall(ctx, q.UserID, q.WorkflowID, q.CallID)
	if err != nil {
		return nil, err
	}
	if call == nil {
		return nil, domainworkflow.ErrCallNotFound
	}
	return h.runs.LatestResponseKeysForCall(ctx, q.UserID, q.WorkflowID, q.CallID)
}
