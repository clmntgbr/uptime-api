package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-api/internal/domain/expiry"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type expiryPolicyRepository struct{ db *gorm.DB }
type expiryItemRepository struct{ db *gorm.DB }
type expiryAlertRepository struct{ db *gorm.DB }

func NewExpiryPolicyRepository(db *gorm.DB) expiry.PolicyRepository {
	return &expiryPolicyRepository{db: db}
}

func NewExpiryItemRepository(db *gorm.DB) expiry.ItemRepository {
	return &expiryItemRepository{db: db}
}

func NewExpiryAlertRepository(db *gorm.DB) expiry.AlertRepository {
	return &expiryAlertRepository{db: db}
}

func (r *expiryPolicyRepository) Get(ctx context.Context, workflowID uuid.UUID, t expiry.Type) (*expiry.Policy, error) {
	var m ExpiryPolicyModel
	err := DBWithContext(ctx, r.db).
		Where("workflow_id = ? AND type = ?", workflowID, string(t)).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return policyFromModel(m)
}

func (r *expiryPolicyRepository) ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) ([]expiry.Policy, error) {
	var models []ExpiryPolicyModel
	err := DBWithContext(ctx, r.db).
		Where("user_id = ? AND workflow_id = ?", userID, workflowID).
		Order("type").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]expiry.Policy, 0, len(models))
	for _, m := range models {
		p, err := policyFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func (r *expiryPolicyRepository) Upsert(ctx context.Context, p expiry.Policy) error {
	raw, err := json.Marshal(p.ThresholdsDays)
	if err != nil {
		return err
	}
	m := ExpiryPolicyModel{
		ID: p.ID, UserID: p.UserID, WorkflowID: p.WorkflowID,
		Type: string(p.Type), Enabled: p.Enabled,
		ThresholdsDays: dbtype.JSONB(raw), UpdatedAt: p.UpdatedAt,
	}
	return DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workflow_id"}, {Name: "type"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "thresholds_days", "updated_at"}),
	}).Create(&m).Error
}

func (r *expiryItemRepository) Upsert(ctx context.Context, item expiry.Item) (expiry.Item, error) {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	meta := item.Meta
	if meta == nil {
		meta = map[string]string{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return expiry.Item{}, err
	}
	m := ExpiryItemModel{
		ID: item.ID, UserID: item.UserID, WorkflowID: item.WorkflowID,
		Type: string(item.Type), Subject: item.Subject, ExpiresAt: item.ExpiresAt,
		Fingerprint: item.Fingerprint, Meta: dbtype.JSONB(raw), ObservedAt: item.ObservedAt,
	}
	err = DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workflow_id"}, {Name: "type"}, {Name: "subject"}},
		DoUpdates: clause.Assignments(map[string]any{
			"expires_at":   m.ExpiresAt,
			"fingerprint":  m.Fingerprint,
			"meta":         m.Meta,
			"observed_at":  m.ObservedAt,
			"id":           gorm.Expr("expiry_items.id"),
		}),
	}).Create(&m).Error
	if err != nil {
		return expiry.Item{}, err
	}
	// Reload to get stable id after conflict.
	var stored ExpiryItemModel
	err = DBWithContext(ctx, r.db).
		Where("workflow_id = ? AND type = ? AND subject = ?", item.WorkflowID, string(item.Type), item.Subject).
		Take(&stored).Error
	if err != nil {
		return expiry.Item{}, err
	}
	return itemFromModel(stored)
}

func (r *expiryItemRepository) ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID, t expiry.Type) ([]expiry.Item, error) {
	var models []ExpiryItemModel
	err := DBWithContext(ctx, r.db).
		Where("user_id = ? AND workflow_id = ? AND type = ?", userID, workflowID, string(t)).
		Order("expires_at").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]expiry.Item, 0, len(models))
	for _, m := range models {
		item, err := itemFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *expiryItemRepository) ListAllForScan(ctx context.Context) ([]expiry.Item, error) {
	var models []ExpiryItemModel
	err := DBWithContext(ctx, r.db).Order("expires_at").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]expiry.Item, 0, len(models))
	for _, m := range models {
		item, err := itemFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *expiryAlertRepository) Claim(ctx context.Context, itemID uuid.UUID, fingerprint string, thresholds []int) ([]int, error) {
	if len(thresholds) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	claimed := make([]int, 0, len(thresholds))
	for _, t := range thresholds {
		res := DBWithContext(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&ExpiryAlertModel{
			ItemID: itemID, ThresholdDays: t, Fingerprint: fingerprint, SentAt: now,
		})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 1 {
			claimed = append(claimed, t)
		}
	}
	return claimed, nil
}

func policyFromModel(m ExpiryPolicyModel) (*expiry.Policy, error) {
	var days []int
	if len(m.ThresholdsDays) > 0 {
		if err := json.Unmarshal(m.ThresholdsDays, &days); err != nil {
			return nil, err
		}
	}
	return &expiry.Policy{
		ID: m.ID, UserID: m.UserID, WorkflowID: m.WorkflowID,
		Type: expiry.Type(m.Type), Enabled: m.Enabled,
		ThresholdsDays: days, UpdatedAt: m.UpdatedAt,
	}, nil
}

func itemFromModel(m ExpiryItemModel) (expiry.Item, error) {
	meta := map[string]string{}
	if len(m.Meta) > 0 {
		if err := json.Unmarshal(m.Meta, &meta); err != nil {
			return expiry.Item{}, err
		}
	}
	return expiry.Item{
		ID: m.ID, UserID: m.UserID, WorkflowID: m.WorkflowID,
		Type: expiry.Type(m.Type), Subject: m.Subject, ExpiresAt: m.ExpiresAt,
		Fingerprint: m.Fingerprint, Meta: meta, ObservedAt: m.ObservedAt,
	}, nil
}
