package incident

import (
	"time"

	"github.com/google/uuid"
)

type Incident struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	WorkflowID       uuid.UUID
	Kind             Kind
	CallID           *uuid.UUID
	Fingerprint      string
	Severity         Severity
	State            State
	Cause            map[string]any
	OpenedAt         time.Time
	AcknowledgedAt   *time.Time
	AcknowledgedBy   string
	ResolvedAt       *time.Time
	ReopenCount      int
	OpenedByRunID    *uuid.UUID
	LastFailureRunID *uuid.UUID
	LastReminderAt   *time.Time
	ReminderCount    int
}

type Event struct {
	ID         uuid.UUID
	IncidentID uuid.UUID
	At         time.Time
	Type       EventType
	RunID      *uuid.UUID
	Payload    map[string]any
}

// ApplyReopen mutates a resolved incident back to open after a failing signal.
func (inc *Incident) ApplyReopen(now time.Time, cause map[string]any, runID *uuid.UUID, severity Severity) {
	inc.State = StateOpen
	inc.Cause = cause
	inc.Severity = severity
	inc.ReopenCount++
	inc.ReminderCount = 0
	inc.LastReminderAt = nil
	inc.AcknowledgedAt = nil
	inc.AcknowledgedBy = ""
	inc.ResolvedAt = nil
	inc.OpenedAt = now
	inc.LastFailureRunID = runID
}

func (inc *Incident) Acknowledge(now time.Time, by string) bool {
	if inc.State != StateOpen {
		return false // already ack or resolved — idempotent no-op for ack→ack handled by caller
	}
	inc.State = StateAcknowledged
	t := now.UTC()
	inc.AcknowledgedAt = &t
	inc.AcknowledgedBy = by
	return true
}

func (inc *Incident) Resolve(now time.Time) {
	inc.State = StateResolved
	t := now.UTC()
	inc.ResolvedAt = &t
}
