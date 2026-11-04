package incident

import (
	"context"

	domainincident "go-api/internal/domain/incident"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListPoliciesQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type ListPoliciesHandler struct {
	workflows domainworkflow.ReadRepository
	policies  domainincident.PolicyRepository
}

func NewListPoliciesHandler(
	workflows domainworkflow.ReadRepository,
	policies domainincident.PolicyRepository,
) *ListPoliciesHandler {
	return &ListPoliciesHandler{workflows: workflows, policies: policies}
}

func (h *ListPoliciesHandler) Handle(ctx context.Context, q ListPoliciesQuery) ([]domainincident.Policy, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, domainworkflow.ErrNotFound
	}
	stored, err := h.policies.ListByWorkflow(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	byKind := map[domainincident.Kind]domainincident.Policy{}
	for _, p := range stored {
		byKind[p.Kind] = p
	}
	out := make([]domainincident.Policy, 0, len(domainincident.AllKinds))
	for _, kind := range domainincident.AllKinds {
		if p, ok := byKind[kind]; ok {
			out = append(out, p)
			continue
		}
		out = append(out, domainincident.DefaultPolicy(q.UserID, q.WorkflowID, kind))
	}
	return out, nil
}
