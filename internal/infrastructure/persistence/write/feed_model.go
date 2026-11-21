package write

import (
	"time"

	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

type FeedEventModel struct {
	ID         uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID     uuid.UUID    `gorm:"column:user_id"`
	WorkflowID *uuid.UUID   `gorm:"column:workflow_id"`
	RunID      *uuid.UUID   `gorm:"column:run_id"`
	CallID     *uuid.UUID   `gorm:"column:call_id"`
	IncidentID *uuid.UUID   `gorm:"column:incident_id"`
	Kind       string       `gorm:"column:kind"`
	Severity   string       `gorm:"column:severity"`
	At         time.Time    `gorm:"column:at"`
	Title      string       `gorm:"column:title"`
	Payload    dbtype.JSONB `gorm:"column:payload"`
}

func (FeedEventModel) TableName() string { return "feed_events" }
