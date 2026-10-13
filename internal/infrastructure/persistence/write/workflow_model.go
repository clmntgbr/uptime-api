package write

import (
	"time"

	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

// Persistence mappings for the workflow configuration tables.
// Schema is owned by SQL migrations — do not encode DDL in tags.

type WorkflowModel struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	UserID          uuid.UUID  `gorm:"column:user_id"`
	Name            string     `gorm:"column:name"`
	IntervalSeconds int        `gorm:"column:interval_seconds"`
	TimeoutMS       int        `gorm:"column:timeout_ms"`
	Enabled         bool       `gorm:"column:enabled"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (WorkflowModel) TableName() string { return "workflows" }

type CallModel struct {
	ID          uuid.UUID    `gorm:"column:id;primaryKey"`
	WorkflowID  uuid.UUID    `gorm:"column:workflow_id"`
	Position    int          `gorm:"column:position"`
	Name        string       `gorm:"column:name"`
	Method      string       `gorm:"column:method"`
	URL         string       `gorm:"column:url"`
	Headers     dbtype.JSONB `gorm:"column:headers"`
	Query       dbtype.JSONB `gorm:"column:query"`
	Body        *string      `gorm:"column:body"`
	TimeoutMS   int          `gorm:"column:timeout_ms"`
	VerifyTLS   bool         `gorm:"column:verify_tls"`
	IPVersion   string       `gorm:"column:ip_version"`
	ProxyURL    *string      `gorm:"column:proxy_url"`
	Extractions dbtype.JSONB `gorm:"column:extractions"`
	CreatedAt   time.Time    `gorm:"column:created_at"`
	UpdatedAt   time.Time    `gorm:"column:updated_at"`
}

func (CallModel) TableName() string { return "calls" }

type RuleModel struct {
	ID        uuid.UUID `gorm:"column:id;primaryKey"`
	CallID    uuid.UUID `gorm:"column:call_id"`
	Position  int       `gorm:"column:position"`
	Subject   string    `gorm:"column:subject"`
	Operator  string    `gorm:"column:operator"`
	Target    *string   `gorm:"column:target"`
	Value     *string   `gorm:"column:value"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (RuleModel) TableName() string { return "rules" }

type VariableModel struct {
	ID         uuid.UUID  `gorm:"column:id;primaryKey"`
	UserID     uuid.UUID  `gorm:"column:user_id"`
	WorkflowID *uuid.UUID `gorm:"column:workflow_id"`
	Type       string     `gorm:"column:type"`
	Name       string     `gorm:"column:name"`
	Value      string     `gorm:"column:value"`
	Secret     bool       `gorm:"column:secret"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
}

func (VariableModel) TableName() string { return "variables" }

type WorkflowStateModel struct {
	WorkflowID           uuid.UUID  `gorm:"column:workflow_id;primaryKey"`
	Status               string     `gorm:"column:status"`
	NextRunAt            time.Time  `gorm:"column:next_run_at"`
	LeaseRunID           *uuid.UUID `gorm:"column:lease_run_id"`
	LeaseStartedAt       *time.Time `gorm:"column:lease_started_at"`
	LeaseUntil           *time.Time `gorm:"column:lease_until"`
	ConsecutiveFailures  int        `gorm:"column:consecutive_failures"`
	ConsecutiveSuccesses int        `gorm:"column:consecutive_successes"`
	LastRunID            *uuid.UUID `gorm:"column:last_run_id"`
	LastRunAt            *time.Time `gorm:"column:last_run_at"`
	LastStatus           *string    `gorm:"column:last_status"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (WorkflowStateModel) TableName() string { return "workflow_state" }
