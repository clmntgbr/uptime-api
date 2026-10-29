package expiry

import (
	"context"

	domainexpiry "go-api/internal/domain/expiry"
	cmdexpiry "go-api/internal/application/command/expiry"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListPoliciesQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type ListPoliciesHandler struct {
	workflows domainworkflow.ReadRepository
	policies  domainexpiry.PolicyRepository
}

func NewListPoliciesHandler(
	workflows domainworkflow.ReadRepository,
	policies domainexpiry.PolicyRepository,
) *ListPoliciesHandler {
	return &ListPoliciesHandler{workflows: workflows, policies: policies}
}

func (h *ListPoliciesHandler) Handle(ctx context.Context, q ListPoliciesQuery) ([]domainexpiry.Policy, error) {
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
	byType := map[domainexpiry.Type]domainexpiry.Policy{}
	for _, p := range stored {
		byType[p.Type] = p
	}
	// Phase 1: always expose tls_certificate (defaults if missing).
	if _, ok := byType[domainexpiry.TypeTLSCertificate]; !ok {
		p, err := cmdexpiry.ResolvePolicy(ctx, h.policies, q.UserID, q.WorkflowID, domainexpiry.TypeTLSCertificate)
		if err != nil {
			return nil, err
		}
		byType[domainexpiry.TypeTLSCertificate] = p
	}
	out := make([]domainexpiry.Policy, 0, len(byType))
	for _, p := range byType {
		out = append(out, p)
	}
	return out, nil
}
