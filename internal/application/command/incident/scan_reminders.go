package incident

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainincident "go-api/internal/domain/incident"

	"github.com/google/uuid"
)

type ScanRemindersHandler struct {
	tx        txRunner
	incidents domainincident.IncidentRepository
	events    domainincident.EventRepository
	logger    *slog.Logger
	clock     func() time.Time
}

func NewScanRemindersHandler(
	tx txRunner,
	incidents domainincident.IncidentRepository,
	events domainincident.EventRepository,
	logger *slog.Logger,
) *ScanRemindersHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanRemindersHandler{
		tx: tx, incidents: incidents, events: events,
		logger: logger, clock: time.Now,
	}
}

// Handle sends 1h then 24h reminders for open (unacknowledged) incidents.
// ClaimReminder is conditional on state=open so a concurrent resolve wins.
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
			fresh, err := h.incidents.GetByIDForUpdate(txCtx, inc.UserID, inc.ID)
			if err != nil {
				return err
			}
			if fresh == nil || fresh.State != domainincident.StateOpen {
				return nil
			}
			age := now.Sub(fresh.OpenedAt)
			expected := fresh.ReminderCount
			switch {
			case expected == 0 && age >= domainincident.ReminderAfter:
			case expected == 1 && age >= domainincident.ReminderSecondAfter:
			default:
				return nil
			}
			claimed, err := h.incidents.ClaimReminder(txCtx, fresh.ID, expected, now)
			if err != nil {
				return err
			}
			if !claimed {
				return nil
			}
			if err := h.events.Append(txCtx, domainincident.Event{
				ID: uuid.New(), IncidentID: fresh.ID, At: now,
				Type:    domainincident.EventReminder,
				Payload: map[string]any{"reminderCount": expected + 1},
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
