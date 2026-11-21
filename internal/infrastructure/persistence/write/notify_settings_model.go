package write

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type WorkflowNotificationSettingsModel struct {
	WorkflowID       uuid.UUID      `gorm:"column:workflow_id;primaryKey"`
	UserID           uuid.UUID      `gorm:"column:user_id"`
	Emails           pq.StringArray `gorm:"column:emails;type:text[]"`
	IncidentsEnabled bool           `gorm:"column:incidents_enabled"`
	TLSEnabled       bool           `gorm:"column:tls_enabled"`
	RunsEnabled      bool           `gorm:"column:runs_enabled"`
	RunStatuses      pq.StringArray `gorm:"column:run_statuses;type:text[]"`
	RunMode          string         `gorm:"column:run_mode"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
}

func (WorkflowNotificationSettingsModel) TableName() string {
	return "workflow_notification_settings"
}
