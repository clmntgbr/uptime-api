package write

import (
	"time"

	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

type IncidentPolicyModel struct {
	ID         uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID     uuid.UUID    `gorm:"column:user_id"`
	WorkflowID uuid.UUID    `gorm:"column:workflow_id"`
	Kind       string       `gorm:"column:kind"`
	Enabled    bool         `gorm:"column:enabled"`
	Severity   string       `gorm:"column:severity"`
	OpenAfter  int          `gorm:"column:open_after"`
	CloseAfter int          `gorm:"column:close_after"`
	Params     dbtype.JSONB `gorm:"column:params"`
	UpdatedAt  time.Time    `gorm:"column:updated_at"`
}

func (IncidentPolicyModel) TableName() string { return "incident_policies" }

type SignalStateModel struct {
	WorkflowID           uuid.UUID  `gorm:"column:workflow_id;primaryKey"`
	Fingerprint          string     `gorm:"column:fingerprint;primaryKey"`
	ConsecutiveFailures  int        `gorm:"column:consecutive_failures"`
	ConsecutiveSuccesses int        `gorm:"column:consecutive_successes"`
	LastRunID            *uuid.UUID `gorm:"column:last_run_id"`
	LastSignalAt         *time.Time `gorm:"column:last_signal_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (SignalStateModel) TableName() string { return "signal_state" }

type IncidentModel struct {
	ID               uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID           uuid.UUID    `gorm:"column:user_id"`
	WorkflowID       uuid.UUID    `gorm:"column:workflow_id"`
	Kind             string       `gorm:"column:kind"`
	CallID           *uuid.UUID   `gorm:"column:call_id"`
	Fingerprint      string       `gorm:"column:fingerprint"`
	Severity         string       `gorm:"column:severity"`
	State            string       `gorm:"column:state"`
	Cause            dbtype.JSONB `gorm:"column:cause"`
	OpenedAt         time.Time    `gorm:"column:opened_at"`
	AcknowledgedAt   *time.Time   `gorm:"column:acknowledged_at"`
	AcknowledgedBy   *string      `gorm:"column:acknowledged_by"`
	ResolvedAt       *time.Time   `gorm:"column:resolved_at"`
	ReopenCount      int          `gorm:"column:reopen_count"`
	OpenedByRunID    *uuid.UUID   `gorm:"column:opened_by_run_id"`
	LastFailureRunID *uuid.UUID   `gorm:"column:last_failure_run_id"`
	LastReminderAt   *time.Time   `gorm:"column:last_reminder_at"`
	ReminderCount    int          `gorm:"column:reminder_count"`
}

func (IncidentModel) TableName() string { return "incidents" }

type IncidentEventModel struct {
	ID         uuid.UUID    `gorm:"column:id;primaryKey"`
	IncidentID uuid.UUID    `gorm:"column:incident_id"`
	At         time.Time    `gorm:"column:at"`
	Type       string       `gorm:"column:type"`
	RunID      *uuid.UUID   `gorm:"column:run_id"`
	Payload    dbtype.JSONB `gorm:"column:payload"`
}

func (IncidentEventModel) TableName() string { return "incident_events" }
