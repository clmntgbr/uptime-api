package incident

import (
	"context"
	"fmt"
	"time"

	"go-api/internal/domain/event"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

type AcknowledgeCommand struct {
	UserID     uuid.UUID
	IncidentID uuid.UUID
	By         string
}

type txRunner interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type AcknowledgeHandler struct {
	tx        txRunner
	incidents domainincident.IncidentRepository
	events    domainincident.EventRepository
	outbox    port.OutboxRepository
}

func NewAcknowledgeHandler(
	tx txRunner,
	incidents domainincident.IncidentRepository,
	events domainincident.EventRepository,
	outbox port.OutboxRepository,
) *AcknowledgeHandler {
	return &AcknowledgeHandler{tx: tx, incidents: incidents, events: events, outbox: outbox}
}

func (h *AcknowledgeHandler) Handle(ctx context.Context, cmd AcknowledgeCommand) (domainincident.Incident, error) {
	var out domainincident.Incident
	err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		inc, err := h.incidents.GetByID(txCtx, cmd.UserID, cmd.IncidentID)
		if err != nil {
			return err
		}
		if inc == nil {
			return domainincident.ErrNotFound
		}
		if inc.State == domainincident.StateAcknowledged {
			out = *inc
			return nil
		}
		if inc.State == domainincident.StateResolved {
			return fmt.Errorf("%w: cannot acknowledge a resolved incident", domainincident.ErrInvalidPolicy)
		}
		now := time.Now().UTC()
		if !inc.Acknowledge(now, cmd.By) {
			out = *inc
			return nil
		}
		if err := h.incidents.Update(txCtx, *inc); err != nil {
			return err
		}
		if err := h.events.Append(txCtx, domainincident.Event{
			ID: uuid.New(), IncidentID: inc.ID, At: now,
			Type: domainincident.EventAcknowledged, Payload: map[string]any{"by": cmd.By},
		}); err != nil {
			return err
		}
		if err := h.outbox.StoreEvents(txCtx, []event.DomainEvent{
			domainincident.NewAcknowledgedEvent(*inc, now),
		}); err != nil {
			return err
		}
		out = *inc
		return nil
	})
	return out, err
}
