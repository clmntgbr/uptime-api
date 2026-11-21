package expiry

import (
	"context"
	"fmt"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
	domainfeed "go-api/internal/domain/feed"

	"github.com/google/uuid"
)

type RecordObservationsHandler struct {
	items    domainexpiry.ItemRepository
	evaluate *EvaluateHandler
	feedRepo domainfeed.WriteRepository
}

func NewRecordObservationsHandler(
	items domainexpiry.ItemRepository,
	evaluate *EvaluateHandler,
) *RecordObservationsHandler {
	return &RecordObservationsHandler{items: items, evaluate: evaluate}
}

// WithFeed enables append of tls/expiry threshold feed rows on claim.
func (h *RecordObservationsHandler) WithFeed(repo domainfeed.WriteRepository) *RecordObservationsHandler {
	h.feedRepo = repo
	return h
}

// Handle upserts observations and evaluates each; returns notifications to publish after commit.
func (h *RecordObservationsHandler) Handle(
	ctx context.Context,
	userID, workflowID uuid.UUID,
	observations []domainexpiry.Observation,
) ([]domainexpiry.NotificationRequest, error) {
	if len(observations) == 0 {
		return nil, nil
	}
	var notes []domainexpiry.NotificationRequest
	var feedEvents []domainfeed.Event
	for _, obs := range observations {
		if obs.Subject == "" || obs.Fingerprint == "" || obs.ExpiresAt.IsZero() {
			continue
		}
		observedAt := obs.ObservedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		item, err := h.items.Upsert(ctx, domainexpiry.Item{
			UserID: userID, WorkflowID: workflowID,
			Type: obs.Type, Subject: obs.Subject, ExpiresAt: obs.ExpiresAt,
			Fingerprint: obs.Fingerprint, Meta: obs.Meta, ObservedAt: observedAt,
		})
		if err != nil {
			return nil, err
		}
		outcome, err := h.evaluate.Evaluate(ctx, item)
		if err != nil {
			return nil, err
		}
		if outcome.Notification != nil {
			notes = append(notes, *outcome.Notification)
		}
		feedEvents = append(feedEvents, feedEventsFromOutcome(outcome)...)
	}
	if h.feedRepo != nil && len(feedEvents) > 0 {
		if err := h.feedRepo.Append(ctx, feedEvents); err != nil {
			return nil, err
		}
	}
	return notes, nil
}

func feedEventsFromOutcome(outcome EvaluateOutcome) []domainfeed.Event {
	if outcome.Notification == nil || len(outcome.Claimed) == 0 {
		return nil
	}
	at := outcome.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if outcome.Notification.Kind == "expiry.renewed" {
		wf := outcome.Item.WorkflowID
		return []domainfeed.Event{{
			ID: uuid.New(), UserID: outcome.Item.UserID, WorkflowID: &wf,
			Kind: domainfeed.KindTLSRenewed, Severity: domainfeed.SeveritySuccess, At: at,
			Title: fmt.Sprintf("TLS %s renewed", outcome.Item.Subject),
			Payload: map[string]any{
				"subject": outcome.Item.Subject, "daysLeft": outcome.DaysLeft,
			},
		}}
	}
	out := make([]domainfeed.Event, 0, len(outcome.Claimed))
	for _, th := range outcome.Claimed {
		if outcome.Item.Type == domainexpiry.TypeTLSCertificate {
			out = append(out, domainfeed.TLSThresholdEvent(
				outcome.Item.UserID, outcome.Item.WorkflowID,
				outcome.Item.Subject, outcome.DaysLeft, th, at,
			))
			continue
		}
		wf := outcome.Item.WorkflowID
		out = append(out, domainfeed.Event{
			ID: uuid.New(), UserID: outcome.Item.UserID, WorkflowID: &wf,
			Kind: domainfeed.KindExpiryThresholdCrossed, Severity: domainfeed.SeverityWarning, At: at,
			Title: "Expiry threshold crossed",
			Payload: map[string]any{
				"type": string(outcome.Item.Type), "subject": outcome.Item.Subject,
				"daysLeft": outcome.DaysLeft, "thresholdDays": th,
			},
		})
	}
	return out
}
