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

// EvaluateOutcome is the result of claiming thresholds for one expiry item.
type EvaluateOutcome struct {
	Notification *domainexpiry.NotificationRequest
	Claimed      []int
	DaysLeft     int
	Item         domainexpiry.Item
	At           time.Time
}

// Evaluate loads the policy (or defaults), claims new thresholds, and returns
// at most one notification for the most urgent newly claimed threshold.
func (h *EvaluateHandler) Evaluate(ctx context.Context, item domainexpiry.Item) (EvaluateOutcome, error) {
	out := EvaluateOutcome{Item: item}
	policy, err := h.policies.Get(ctx, item.WorkflowID, item.Type)
	if err != nil {
		return out, err
	}
	if policy == nil {
		def := domainexpiry.DefaultPolicy(item.UserID, item.WorkflowID, item.Type, h.clock().UTC())
		policy = &def
	}
	if !policy.Enabled {
		return out, nil
	}

	now := h.clock().UTC()
	out.At = now
	daysLeft := domainexpiry.DaysLeft(item.ExpiresAt, now)
	out.DaysLeft = daysLeft
	fp := domainexpiry.AlertFingerprint(item.Fingerprint, daysLeft, now)

	var toClaim []int
	switch {
	case daysLeft < -domainexpiry.ExpiredAlertDays:
		return out, nil
	case daysLeft < 0:
		toClaim = []int{0}
	default:
		toClaim = domainexpiry.CrossedThresholds(policy.ThresholdsDays, daysLeft)
	}
	if len(toClaim) == 0 {
		return out, nil
	}

	claimed, err := h.alerts.Claim(ctx, item.ID, fp, toClaim)
	if err != nil {
		return out, err
	}
	out.Claimed = claimed
	if len(claimed) == 0 {
		return out, nil
	}

	notifyAt := domainexpiry.NotifyThreshold(claimed, daysLeft)
	out.Notification = &domainexpiry.NotificationRequest{
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
	}
	return out, nil
}

// ResolvePolicy returns the stored or default policy for a workflow type.
func ResolvePolicy(ctx context.Context, policies domainexpiry.PolicyRepository, userID, workflowID uuid.UUID, t domainexpiry.Type, now time.Time) (domainexpiry.Policy, error) {
	p, err := policies.Get(ctx, workflowID, t)
	if err != nil {
		return domainexpiry.Policy{}, err
	}
	if p == nil {
		return domainexpiry.DefaultPolicy(userID, workflowID, t, now), nil
	}
	return *p, nil
}
