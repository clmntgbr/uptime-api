package expiry

import (
	"context"
	"time"

	domainexpiry "go-api/internal/domain/expiry"

	"github.com/google/uuid"
)

type RecordObservationsHandler struct {
	items    domainexpiry.ItemRepository
	evaluate *EvaluateHandler
}

func NewRecordObservationsHandler(
	items domainexpiry.ItemRepository,
	evaluate *EvaluateHandler,
) *RecordObservationsHandler {
	return &RecordObservationsHandler{items: items, evaluate: evaluate}
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
		note, err := h.evaluate.Evaluate(ctx, item)
		if err != nil {
			return nil, err
		}
		if note != nil {
			notes = append(notes, *note)
		}
	}
	return notes, nil
}
