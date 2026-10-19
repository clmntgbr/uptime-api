package write

import (
	"context"
	"errors"
	"time"

	domainschedule "go-api/internal/domain/schedule"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type stateRepository struct {
	db *gorm.DB
}

func NewStateRepository(db *gorm.DB) domainschedule.Repository {
	return &stateRepository{db: db}
}

func (r *stateRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

type dueRow struct {
	WorkflowStateModel
	UserID          uuid.UUID `gorm:"column:user_id"`
	IntervalSeconds *int      `gorm:"column:interval_seconds"`
	ScheduleType    string    `gorm:"column:schedule_type"`
	Enabled         bool      `gorm:"column:enabled"`
}

// LockDue selects enabled workflows whose next_run_at has passed, and also any
// row with an expired lease (including disabled / parked one-shots) so timeouts
// can be recorded even when the scheduler would not dispatch again.
const lockDueSQL = `
SELECT s.*, w.user_id, w.interval_seconds, w.schedule_type, w.enabled
FROM workflow_state s
JOIN workflows w ON w.id = s.workflow_id
WHERE w.deleted_at IS NULL
  AND (
    (w.enabled AND s.next_run_at <= @now)
    OR (s.status IN ('running', 'cancelling') AND s.lease_until IS NOT NULL AND s.lease_until < @now)
  )
ORDER BY s.next_run_at
LIMIT @limit
FOR UPDATE OF s SKIP LOCKED`

func (r *stateRepository) LockDue(ctx context.Context, now time.Time, limit int) ([]domainschedule.Due, error) {
	var rows []dueRow
	err := DBWithContext(ctx, r.db).
		Raw(lockDueSQL, map[string]any{"now": now.UTC(), "limit": limit}).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domainschedule.Due, 0, len(rows))
	for _, row := range rows {
		due := domainschedule.Due{
			State:        stateDomainFromModel(row.WorkflowStateModel),
			UserID:       row.UserID,
			ScheduleType: row.ScheduleType,
			Enabled:      row.Enabled,
		}
		if row.IntervalSeconds != nil {
			due.Interval = time.Duration(*row.IntervalSeconds) * time.Second
		}
		out = append(out, due)
	}
	return out, nil
}

func (r *stateRepository) GetForUpdate(ctx context.Context, workflowID uuid.UUID) (*domainschedule.State, error) {
	var model WorkflowStateModel
	err := DBWithContext(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&model, "workflow_id = ?", workflowID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	state := stateDomainFromModel(model)
	return &state, nil
}

func (r *stateRepository) Save(ctx context.Context, state domainschedule.State) error {
	return DBWithContext(ctx, r.db).Save(stateModelFromDomain(state)).Error
}

func stateModelFromDomain(s domainschedule.State) *WorkflowStateModel {
	return &WorkflowStateModel{
		WorkflowID:           s.WorkflowID,
		Status:               string(s.Status),
		NextRunAt:            s.NextRunAt.UTC(),
		LeaseRunID:           s.LeaseRunID,
		LeaseStartedAt:       s.LeaseStartedAt,
		LeaseUntil:           s.LeaseUntil,
		ConsecutiveFailures:  s.ConsecutiveFailures,
		ConsecutiveSuccesses: s.ConsecutiveSuccesses,
		LastRunID:            s.LastRunID,
		LastRunAt:            s.LastRunAt,
		LastStatus:           nullableString(s.LastStatus),
		UpdatedAt:            time.Now().UTC(),
	}
}

func stateDomainFromModel(m WorkflowStateModel) domainschedule.State {
	return domainschedule.State{
		WorkflowID:           m.WorkflowID,
		Status:               domainschedule.Status(m.Status),
		NextRunAt:            m.NextRunAt,
		LeaseRunID:           m.LeaseRunID,
		LeaseStartedAt:       m.LeaseStartedAt,
		LeaseUntil:           m.LeaseUntil,
		ConsecutiveFailures:  m.ConsecutiveFailures,
		ConsecutiveSuccesses: m.ConsecutiveSuccesses,
		LastRunID:            m.LastRunID,
		LastRunAt:            m.LastRunAt,
		LastStatus:           derefString(m.LastStatus),
	}
}
