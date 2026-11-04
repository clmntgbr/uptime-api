package incident

import (
	"context"
	"fmt"
	"time"

	domainincident "go-api/internal/domain/incident"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpsertPolicyCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Kind       domainincident.Kind
	Enabled    bool
	Severity   domainincident.Severity
	OpenAfter  int
	CloseAfter int
	Params     map[string]any
}

type UpsertPolicyHandler struct {
	workflows domainworkflow.ReadRepository
	policies  domainincident.PolicyRepository
}

func NewUpsertPolicyHandler(
	workflows domainworkflow.ReadRepository,
	policies domainincident.PolicyRepository,
) *UpsertPolicyHandler {
	return &UpsertPolicyHandler{workflows: workflows, policies: policies}
}

func (h *UpsertPolicyHandler) Handle(ctx context.Context, cmd UpsertPolicyCommand) (domainincident.Policy, error) {
	wf, err := h.workflows.FindByID(ctx, cmd.UserID, cmd.WorkflowID)
	if err != nil {
		return domainincident.Policy{}, err
	}
	if wf == nil {
		return domainincident.Policy{}, domainworkflow.ErrNotFound
	}
	if err := domainincident.ValidatePolicyFields(cmd.Enabled, cmd.Severity, cmd.OpenAfter, cmd.CloseAfter, cmd.Kind, cmd.Params); err != nil {
		return domainincident.Policy{}, err
	}
	existing, err := h.policies.Get(ctx, cmd.WorkflowID, cmd.Kind)
	if err != nil {
		return domainincident.Policy{}, fmt.Errorf("load policy: %w", err)
	}
	id := uuid.New()
	if existing != nil {
		id = existing.ID
	}
	params := cmd.Params
	if params == nil {
		params = map[string]any{}
	}
	p := domainincident.Policy{
		ID: id, UserID: cmd.UserID, WorkflowID: cmd.WorkflowID,
		Kind: cmd.Kind, Enabled: cmd.Enabled, Severity: cmd.Severity,
		OpenAfter: cmd.OpenAfter, CloseAfter: cmd.CloseAfter,
		Params: params, UpdatedAt: time.Now().UTC(),
	}
	if err := h.policies.Upsert(ctx, p); err != nil {
		return domainincident.Policy{}, err
	}
	return p, nil
}
