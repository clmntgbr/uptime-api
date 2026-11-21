package expiry

import (
	"context"
	"fmt"
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
	var kind string
	switch {
	case daysLeft < -domainexpiry.ExpiredAlertDays:
		return out, nil
	case daysLeft < 0:
		toClaim = []int{0}
		kind = "expiry.expired"
	default:
		toClaim = domainexpiry.CrossedThresholds(policy.ThresholdsDays, daysLeft)
		kind = "expiry.threshold"
	}

	// Renewed: nothing currently crossed, and a prior alert used another
	// fingerprint (certificate actually changed — not a policy edit).
	if len(toClaim) == 0 && daysLeft >= 0 {
		hadPrior, err := h.alerts.HadOtherFingerprint(ctx, item.ID, item.Fingerprint)
		if err != nil {
			return out, err
		}
		if !hadPrior {
			return out, nil
		}
		claimed, err := h.alerts.Claim(ctx, item.ID, item.Fingerprint, []int{domainexpiry.RenewedClaimThreshold})
		if err != nil {
			return out, err
		}
		out.Claimed = claimed
		if len(claimed) == 0 {
			return out, nil
		}
		out.Notification = &domainexpiry.NotificationRequest{
			Kind:          "expiry.renewed",
			V:             1,
			Type:          item.Type,
			UserID:        item.UserID,
			WorkflowID:    item.WorkflowID,
			ItemID:        item.ID,
			Subject:       item.Subject,
			DaysLeft:      daysLeft,
			ThresholdDays: domainexpiry.RenewedClaimThreshold,
			ExpiresAt:     item.ExpiresAt,
			Severity:      domainexpiry.SeverityInfo,
			Message:       fmt.Sprintf("The certificate of %s was renewed (expires in %d days)", item.Subject, daysLeft),
		}
		return out, nil
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
	severity := domainexpiry.SeverityFor(daysLeft)
	if kind == "expiry.expired" {
		severity = domainexpiry.SeverityCritical
	}
	out.Notification = &domainexpiry.NotificationRequest{
		Kind:          kind,
		V:             1,
		Type:          item.Type,
		UserID:        item.UserID,
		WorkflowID:    item.WorkflowID,
		ItemID:        item.ID,
		Subject:       item.Subject,
		DaysLeft:      daysLeft,
		ThresholdDays: notifyAt,
		ExpiresAt:     item.ExpiresAt,
		Severity:      severity,
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
