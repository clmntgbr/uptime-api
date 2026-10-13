package read

import (
	"context"
	"errors"
	"strings"
	"time"

	"go-api/internal/domain/paginate"
	domainschedule "go-api/internal/domain/schedule"
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

type workflowListRow struct {
	ID   uuid.UUID `gorm:"column:id"`
	Name string    `gorm:"column:name"`
}

var workflowSortColumns = map[string]string{
	"name":       "name",
	"createdAt":  "created_at",
	"created_at": "created_at",
	"updatedAt":  "updated_at",
	"updated_at": "updated_at",
}

func (r *workflowQueryRepository) List(ctx context.Context, userID uuid.UUID, q paginate.PaginateQuery) ([]domainworkflow.ListView, int64, error) {
	column, ok := workflowSortColumns[q.SortBy]
	if !ok {
		column = "created_at"
	}
	q.SortBy = column

	base := r.db.WithContext(ctx).
		Table("workflows").
		Where("user_id = ? AND deleted_at IS NULL", userID)
	if search := strings.TrimSpace(q.Search); search != "" {
		base = base.Where("name ILIKE ?", "%"+escapeLike(search)+"%")
	}

	scoped, total, err := Paginate(base, q)
	if err != nil {
		return nil, 0, err
	}
	var rows []workflowListRow
	if err := scoped.Select("id, name").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	views := make([]domainworkflow.ListView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domainworkflow.ListView{ID: row.ID, Name: row.Name})
	}
	return views, total, nil
}

// escapeLike makes % and _ typed by the user match literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *workflowQueryRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*domainworkflow.View, error) {
	var row workflowRow
	err := r.db.WithContext(ctx).
		Table("workflows w").
		Select("w.id, w.user_id, w.name, w.interval_seconds, w.enabled, w.created_at, w.updated_at, s.status, s.next_run_at, s.last_run_at, s.last_status").
		Joins("LEFT JOIN workflow_state s ON s.workflow_id = w.id").
		Where("w.id = ? AND w.user_id = ? AND w.deleted_at IS NULL", id, userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	view := &domainworkflow.View{
		ID:              row.ID,
		UserID:          row.UserID,
		Name:            row.Name,
		IntervalSeconds: row.IntervalSeconds,
		Enabled:         row.Enabled,
		NextRunAt:       row.NextRunAt,
		LastRunAt:       row.LastRunAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.Status != nil {
		view.Status = domainschedule.Status(*row.Status)
	}
	if row.LastStatus != nil {
		view.LastStatus = *row.LastStatus
	}
	return view, nil
}
