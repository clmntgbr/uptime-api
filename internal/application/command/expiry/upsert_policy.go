package expiry

import (
	"context"
	"fmt"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpsertPolicyCommand struct {
	UserID         uuid.UUID
	WorkflowID     uuid.UUID
	Type           domainexpiry.Type
	Enabled        bool
	ThresholdsDays []int
}

type UpsertPolicyHandler struct {
	workflows domainworkflow.ReadRepository
	policies  domainexpiry.PolicyRepository
}

func NewUpsertPolicyHandler(
	workflows domainworkflow.ReadRepository,
	policies domainexpiry.PolicyRepository,
) *UpsertPolicyHandler {
	return &UpsertPolicyHandler{workflows: workflows, policies: policies}
}

func (h *UpsertPolicyHandler) Handle(ctx context.Context, cmd UpsertPolicyCommand) (domainexpiry.Policy, error) {
	wf, err := h.workflows.FindByID(ctx, cmd.UserID, cmd.WorkflowID)
	if err != nil {
		return domainexpiry.Policy{}, err
	}
	if wf == nil {
		return domainexpiry.Policy{}, domainworkflow.ErrNotFound
	}
	if !cmd.Enabled {
		// Persist disabled even with empty thresholds (use defaults stored for reactivation).
		thresholds := cmd.ThresholdsDays
		if len(thresholds) == 0 {
			thresholds = append([]int(nil), domainexpiry.DefaultThresholdsDays...)
		} else {
			thresholds, err = domainexpiry.NormalizeThresholds(thresholds)
			if err != nil {
				return domainexpiry.Policy{}, err
			}
		}
		return h.save(ctx, cmd, thresholds)
	}
	thresholds, err := domainexpiry.NormalizeThresholds(cmd.ThresholdsDays)
	if err != nil {
		return domainexpiry.Policy{}, err
	}
	return h.save(ctx, cmd, thresholds)
}

func (h *UpsertPolicyHandler) save(ctx context.Context, cmd UpsertPolicyCommand, thresholds []int) (domainexpiry.Policy, error) {
	existing, err := h.policies.Get(ctx, cmd.WorkflowID, cmd.Type)
	if err != nil {
		return domainexpiry.Policy{}, fmt.Errorf("load policy: %w", err)
	}
	id := uuid.New()
	if existing != nil {
		id = existing.ID
	}
	p := domainexpiry.Policy{
		ID: id, UserID: cmd.UserID, WorkflowID: cmd.WorkflowID,
		Type: cmd.Type, Enabled: cmd.Enabled, ThresholdsDays: thresholds,
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.policies.Upsert(ctx, p); err != nil {
		return domainexpiry.Policy{}, err
	}
	return p, nil
}
