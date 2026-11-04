package incident

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

type ScanRemindersHandler struct {
	tx        txRunner
	incidents domainincident.IncidentRepository
	events    domainincident.EventRepository
	outbox    port.OutboxRepository
	logger    *slog.Logger
	clock     func() time.Time
}

func NewScanRemindersHandler(
	tx txRunner,
	incidents domainincident.IncidentRepository,
	events domainincident.EventRepository,
	outbox port.OutboxRepository,
	logger *slog.Logger,
) *ScanRemindersHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanRemindersHandler{
		tx: tx, incidents: incidents, events: events, outbox: outbox,
		logger: logger, clock: time.Now,
	}
}

// Handle sends 1h then 24h reminders for open (unacknowledged) incidents.
func (h *ScanRemindersHandler) Handle(ctx context.Context) (ApplyResult, error) {
	var result ApplyResult
	now := h.clock().UTC()
	open, err := h.incidents.ListOpenForReminders(ctx, now)
	if err != nil {
		return result, err
	}
	for _, inc := range open {
		var note NotifyIntent
		err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			fresh, err := h.incidents.GetByID(txCtx, inc.UserID, inc.ID)
			if err != nil {
				return err
			}
			if fresh == nil || fresh.State != domainincident.StateOpen {
				return nil
			}
			age := now.Sub(fresh.OpenedAt)
			switch {
			case fresh.ReminderCount == 0 && age >= domainincident.ReminderAfter:
			case fresh.ReminderCount == 1 && age >= domainincident.ReminderSecondAfter:
			default:
				return nil
			}
			fresh.ReminderCount++
			t := now
			fresh.LastReminderAt = &t
			if err := h.incidents.Update(txCtx, *fresh); err != nil {
				return err
			}
			if err := h.events.Append(txCtx, domainincident.Event{
				ID: uuid.New(), IncidentID: fresh.ID, At: now,
				Type:    domainincident.EventReminder,
				Payload: map[string]any{"reminderCount": fresh.ReminderCount},
			}); err != nil {
				return err
			}
			note = NotifyIntent{
				Kind: "incident.reminder", IncidentID: fresh.ID,
				UserID: fresh.UserID, WorkflowID: fresh.WorkflowID,
				IncidentKind: fresh.Kind, Severity: fresh.Severity,
				Message: fmt.Sprintf("Incident %s still open", fresh.Kind),
			}
			return nil
		})
		if err != nil {
			h.logger.Error("reminder failed", "incident_id", inc.ID, "error", err)
			continue
		}
		if note.IncidentID != uuid.Nil {
			result.Notifications = append(result.Notifications, note)
		}
	}
	return result, nil
}
