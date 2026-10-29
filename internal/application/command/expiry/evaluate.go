package expiry

import (
	"context"
	"time"

	domainexpiry "go-api/internal/domain/expiry"

	"github.com/google/uuid"
)

type EvaluateHandler struct {
	policies domainexpiry.PolicyRepository
	alerts   domainexpiry.AlertRepository
	clock    func() time.Time
}

func NewEvaluateHandler(
	policies domainexpiry.PolicyRepository,
	alerts domainexpiry.AlertRepository,
) *EvaluateHandler {
	return &EvaluateHandler{policies: policies, alerts: alerts, clock: time.Now}
}

// Evaluate loads the policy (or defaults), claims new thresholds, and returns
// at most one notification for the most urgent newly claimed threshold.
func (h *EvaluateHandler) Evaluate(ctx context.Context, item domainexpiry.Item) (*domainexpiry.NotificationRequest, error) {
	policy, err := h.policies.Get(ctx, item.WorkflowID, item.Type)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		def := domainexpiry.DefaultPolicy(item.UserID, item.WorkflowID, item.Type)
		policy = &def
	}
	if !policy.Enabled {
		return nil, nil
	}

	now := h.clock().UTC()
	daysLeft := domainexpiry.DaysLeft(item.ExpiresAt, now)
	fp := domainexpiry.AlertFingerprint(item.Fingerprint, daysLeft, now)

	var toClaim []int
	switch {
	case daysLeft < -domainexpiry.ExpiredAlertDays:
		return nil, nil
	case daysLeft < 0:
		toClaim = []int{0}
	default:
		toClaim = domainexpiry.CrossedThresholds(policy.ThresholdsDays, daysLeft)
	}
	if len(toClaim) == 0 {
		return nil, nil
	}

	claimed, err := h.alerts.Claim(ctx, item.ID, fp, toClaim)
	if err != nil {
		return nil, err
	}
	if len(claimed) == 0 {
		return nil, nil
	}

	notifyAt := domainexpiry.NotifyThreshold(claimed, daysLeft)
	return &domainexpiry.NotificationRequest{
		Kind:          "expiry.threshold",
		V:             1,
		Type:          item.Type,
		UserID:        item.UserID,
		WorkflowID:    item.WorkflowID,
		ItemID:        item.ID,
		Subject:       item.Subject,
		DaysLeft:      daysLeft,
		ThresholdDays: notifyAt,
		ExpiresAt:     item.ExpiresAt,
		Severity:      domainexpiry.SeverityFor(daysLeft),
		Message:       domainexpiry.MessageFor(item.Type, item.Subject, daysLeft),
	}, nil
}

// ResolvePolicy returns the stored or default policy for a workflow type.
func ResolvePolicy(ctx context.Context, policies domainexpiry.PolicyRepository, userID, workflowID uuid.UUID, t domainexpiry.Type) (domainexpiry.Policy, error) {
	p, err := policies.Get(ctx, workflowID, t)
	if err != nil {
		return domainexpiry.Policy{}, err
	}
	if p == nil {
		return domainexpiry.DefaultPolicy(userID, workflowID, t), nil
	}
	return *p, nil
}
