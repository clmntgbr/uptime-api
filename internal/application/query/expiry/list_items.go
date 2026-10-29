package expiry

import (
	"context"
	"time"

	cmdexpiry "go-api/internal/application/command/expiry"
	domainexpiry "go-api/internal/domain/expiry"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListItemsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Type       domainexpiry.Type
}

type ListItemsHandler struct {
	workflows domainworkflow.ReadRepository
	policies  domainexpiry.PolicyRepository
	items     domainexpiry.ItemRepository
	clock     func() time.Time
}

func NewListItemsHandler(
	workflows domainworkflow.ReadRepository,
	policies domainexpiry.PolicyRepository,
	items domainexpiry.ItemRepository,
) *ListItemsHandler {
	return &ListItemsHandler{
		workflows: workflows, policies: policies, items: items, clock: time.Now,
	}
}

func (h *ListItemsHandler) Handle(ctx context.Context, q ListItemsQuery) ([]domainexpiry.ItemView, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, domainworkflow.ErrNotFound
	}
	policy, err := cmdexpiry.ResolvePolicy(ctx, h.policies, q.UserID, q.WorkflowID, q.Type)
	if err != nil {
		return nil, err
	}
	items, err := h.items.ListByWorkflow(ctx, q.UserID, q.WorkflowID, q.Type)
	if err != nil {
		return nil, err
	}
	now := h.clock().UTC()
	out := make([]domainexpiry.ItemView, 0, len(items))
	for _, item := range items {
		state, days, next := domainexpiry.ItemViewState(item, policy.ThresholdsDays, now)
		view := domainexpiry.ItemView{
			Item: item, DaysLeft: days, State: state, NextThresholdDays: next,
		}
		if item.Meta != nil {
			view.Issuer = item.Meta["issuer"]
		}
		out = append(out, view)
	}
	return out, nil
}
