package write

import (
	"time"

	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

// Persistence mappings for the partitioned result tables and their aggregates.

type RunModel struct {
	ID           uuid.UUID    `gorm:"column:id;primaryKey"`
	WorkflowID   uuid.UUID    `gorm:"column:workflow_id"`
	UserID       uuid.UUID    `gorm:"column:user_id"`
	Trigger      string       `gorm:"column:trigger"`
	Status       string       `gorm:"column:status"`
	Reason       *string      `gorm:"column:reason"`
	StartedAt    time.Time    `gorm:"column:started_at;primaryKey"`
	FinishedAt   time.Time    `gorm:"column:finished_at"`
	DurationMS   int          `gorm:"column:duration_ms"`
	CallsTotal   int          `gorm:"column:calls_total"`
	CallsSuccess int          `gorm:"column:calls_success"`
	CallsFailed  int          `gorm:"column:calls_failed"`
	CallsError   int          `gorm:"column:calls_error"`
	CallsSkipped int          `gorm:"column:calls_skipped"`
	Variables    dbtype.JSONB `gorm:"column:variables"`
	RecordedAt   time.Time    `gorm:"column:recorded_at"`
}

func (RunModel) TableName() string { return "runs" }

type CallResultModel struct {
	ID                 uuid.UUID    `gorm:"column:id;primaryKey"`
	RunID              uuid.UUID    `gorm:"column:run_id"`
	RunStartedAt       time.Time    `gorm:"column:run_started_at;primaryKey"`
	WorkflowID         uuid.UUID    `gorm:"column:workflow_id"`
	CallID             *uuid.UUID   `gorm:"column:call_id"`
	Name               string       `gorm:"column:name"`
	Position           int          `gorm:"column:position"`
	Status             string       `gorm:"column:status"`
	Reason             *string      `gorm:"column:reason"`
	HTTPStatus         *int         `gorm:"column:http_status"`
	Request            dbtype.JSONB `gorm:"column:request"`
	Response           dbtype.JSONB `gorm:"column:response"`
	Timings            dbtype.JSONB `gorm:"column:timings"`
	Network            dbtype.JSONB `gorm:"column:network"`
	TLS                dbtype.JSONB `gorm:"column:tls"`
	ExtractedVariables dbtype.JSONB `gorm:"column:extracted_variables"`
	UsedVariables      dbtype.JSONB `gorm:"column:used_variables"`
	ErrorType          *string      `gorm:"column:error_type"`
	ErrorMessage       *string      `gorm:"column:error_message"`
	BodyRef            *string      `gorm:"column:body_ref"`
	Truncated          bool         `gorm:"column:truncated"`
	StartedAt          *time.Time   `gorm:"column:started_at"`
	FinishedAt         *time.Time   `gorm:"column:finished_at"`
}

func (CallResultModel) TableName() string { return "call_results" }

type RuleResultModel struct {
	ID           uuid.UUID `gorm:"column:id;primaryKey"`
	CallResultID uuid.UUID `gorm:"column:call_result_id"`
	RunID        uuid.UUID `gorm:"column:run_id"`
	RunStartedAt time.Time `gorm:"column:run_started_at;primaryKey"`
	Position     int       `gorm:"column:position"`
	Subject      string    `gorm:"column:subject"`
	Operator     string    `gorm:"column:operator"`
	Text         string    `gorm:"column:text"`
	Observed     string    `gorm:"column:observed"`
	Expected     string    `gorm:"column:expected"`
	Passed       bool      `gorm:"column:passed"`
}

func (RuleResultModel) TableName() string { return "rule_results" }

type CertificateModel struct {
	ID             uuid.UUID `gorm:"column:id;primaryKey"`
	UserID         uuid.UUID `gorm:"column:user_id"`
	Host           string    `gorm:"column:host"`
	Port           int       `gorm:"column:port"`
	Subject        string    `gorm:"column:subject"`
	Issuer         string    `gorm:"column:issuer"`
	SerialNumber   string    `gorm:"column:serial_number"`
	ExpiresAt      time.Time `gorm:"column:expires_at"`
	ChainExpiresAt time.Time `gorm:"column:chain_expires_at"`
	ChainValid     bool      `gorm:"column:chain_valid"`
	NameMatch      bool      `gorm:"column:name_match"`
	FirstSeenAt    time.Time `gorm:"column:first_seen_at"`
	LastSeenAt     time.Time `gorm:"column:last_seen_at"`
}

func (CertificateModel) TableName() string { return "certificates" }

type AggregateColumns struct {
	WorkflowID   uuid.UUID `gorm:"column:workflow_id;primaryKey"`
	BucketStart  time.Time `gorm:"column:bucket_start;primaryKey"`
	RunsTotal    int       `gorm:"column:runs_total"`
	RunsSuccess  int       `gorm:"column:runs_success"`
	Availability float64   `gorm:"column:availability"`
	LatencyP50MS *int      `gorm:"column:latency_p50_ms"`
	LatencyP95MS *int      `gorm:"column:latency_p95_ms"`
	ComputedAt   time.Time `gorm:"column:computed_at"`
}

type HourlyAggregateModel struct {
	AggregateColumns `gorm:"embedded"`
}

func (HourlyAggregateModel) TableName() string { return "hourly_aggregates" }

type DailyAggregateModel struct {
	AggregateColumns `gorm:"embedded"`
}

func (DailyAggregateModel) TableName() string { return "daily_aggregates" }
