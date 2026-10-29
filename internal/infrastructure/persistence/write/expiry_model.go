package write

import (
	"time"

	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

type ExpiryPolicyModel struct {
	ID             uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID         uuid.UUID    `gorm:"column:user_id"`
	WorkflowID     uuid.UUID    `gorm:"column:workflow_id"`
	Type           string       `gorm:"column:type"`
	Enabled        bool         `gorm:"column:enabled"`
	ThresholdsDays dbtype.JSONB `gorm:"column:thresholds_days"`
	UpdatedAt      time.Time    `gorm:"column:updated_at"`
}

func (ExpiryPolicyModel) TableName() string { return "expiry_policies" }

type ExpiryItemModel struct {
	ID          uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID      uuid.UUID    `gorm:"column:user_id"`
	WorkflowID  uuid.UUID    `gorm:"column:workflow_id"`
	Type        string       `gorm:"column:type"`
	Subject     string       `gorm:"column:subject"`
	ExpiresAt   time.Time    `gorm:"column:expires_at"`
	Fingerprint string       `gorm:"column:fingerprint"`
	Meta        dbtype.JSONB `gorm:"column:meta"`
	ObservedAt  time.Time    `gorm:"column:observed_at"`
}

func (ExpiryItemModel) TableName() string { return "expiry_items" }

type ExpiryAlertModel struct {
	ItemID         uuid.UUID `gorm:"column:item_id;primaryKey"`
	ThresholdDays  int       `gorm:"column:threshold_days;primaryKey"`
	Fingerprint    string    `gorm:"column:fingerprint;primaryKey"`
	SentAt         time.Time `gorm:"column:sent_at"`
}

func (ExpiryAlertModel) TableName() string { return "expiry_alerts" }
