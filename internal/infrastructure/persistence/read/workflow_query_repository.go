package read

import (
	"context"
	"errors"
	"time"

	domainschedule "go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowQueryRepository struct {
	db *gorm.DB
}

func NewWorkflowQueryRepository(db *gorm.DB) domainworkflow.ReadRepository {
	return &workflowQueryRepository{db: db}
}

type workflowRow struct {
	ID              uuid.UUID  `gorm:"column:id"`
	UserID          uuid.UUID  `gorm:"column:user_id"`
	Name            string     `gorm:"column:name"`
	IntervalSeconds int        `gorm:"column:interval_seconds"`
	Enabled         bool       `gorm:"column:enabled"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	Status          *string    `gorm:"column:status"`
	NextRunAt       *time.Time `gorm:"column:next_run_at"`
	LastRunAt       *time.Time `gorm:"column:last_run_at"`
	LastStatus      *string    `gorm:"column:last_status"`
}

type callRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	Position  int       `gorm:"column:position"`
	Name      string    `gorm:"column:name"`
	Method    string    `gorm:"column:method"`
	URL       string    `gorm:"column:url"`
	RuleCount int       `gorm:"column:rule_count"`
}

func (r *workflowQueryRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*domainworkflow.View, error) {
	var row workflowRow
	err := r.db.WithContext(ctx).
		Table("workflows w").
		Select("w.id, w.user_id, w.name, w.interval_seconds, w.enabled, w.created_at, w.updated_at, s.status, s.next_run_at, s.last_run_at, s.last_status").
		Joins("LEFT JOIN workflow_state s ON s.workflow_id = w.id").
		Where("w.id = ? AND w.user_id = ?", id, userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var calls []callRow
	err = r.db.WithContext(ctx).
		Table("calls c").
		Select("c.id, c.position, c.name, c.method, c.url, COUNT(r.id) AS rule_count").
		Joins("LEFT JOIN rules r ON r.call_id = c.id").
		Where("c.workflow_id = ?", id).
		Group("c.id").
		Order("c.position").
		Scan(&calls).Error
	if err != nil {
		return nil, err
	}

	var statics []string
	err = r.db.WithContext(ctx).
		Table("variables").
		Where("workflow_id = ? AND type = ?", id, string(variable.TypeStatic)).
		Order("name").
		Pluck("name", &statics).Error
	if err != nil {
		return nil, err
	}

	view := &domainworkflow.View{
		ID:              row.ID,
		UserID:          row.UserID,
		Name:            row.Name,
		IntervalSeconds: row.IntervalSeconds,
		Enabled:         row.Enabled,
		StaticNames:     statics,
		NextRunAt:       row.NextRunAt,
		LastRunAt:       row.LastRunAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Calls:           make([]domainworkflow.CallView, 0, len(calls)),
	}
	if row.Status != nil {
		view.Status = domainschedule.Status(*row.Status)
	}
	if row.LastStatus != nil {
		view.LastStatus = *row.LastStatus
	}
	for _, c := range calls {
		view.Calls = append(view.Calls, domainworkflow.CallView{
			ID: c.ID, Position: c.Position, Name: c.Name, Method: c.Method, URL: c.URL, RuleCount: c.RuleCount,
		})
	}
	return view, nil
}
