package write

import (
	"context"
	"time"

	"go-api/internal/domain/incident"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type incidentScanRepository struct{ db *gorm.DB }

func NewIncidentScanRepository(db *gorm.DB) incident.ScanRepository {
	return &incidentScanRepository{db: db}
}

type enabledWorkflowRow struct {
	UserID          uuid.UUID  `gorm:"column:user_id"`
	WorkflowID      uuid.UUID  `gorm:"column:workflow_id"`
	IntervalSeconds *int       `gorm:"column:interval_seconds"`
	LastRunAt       *time.Time `gorm:"column:last_run_at"`
	EnabledAt       time.Time  `gorm:"column:enabled_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
}

func (r *incidentScanRepository) ListEnabledIntervalWorkflows(ctx context.Context) ([]incident.EnabledWorkflow, error) {
	var rows []enabledWorkflowRow
	err := DBWithContext(ctx, r.db).Raw(`
SELECT w.user_id, w.id AS workflow_id, w.interval_seconds, s.last_run_at, w.enabled_at, w.created_at
FROM workflows w
JOIN workflow_state s ON s.workflow_id = w.id
WHERE w.deleted_at IS NULL
  AND w.enabled = TRUE
  AND w.schedule_type = 'interval'
  AND w.interval_seconds IS NOT NULL
  AND w.interval_seconds > 0
`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]incident.EnabledWorkflow, 0, len(rows))
	for _, row := range rows {
		wf := incident.EnabledWorkflow{
			UserID: row.UserID, WorkflowID: row.WorkflowID,
			LastRunAt: row.LastRunAt, EnabledAt: row.EnabledAt, CreatedAt: row.CreatedAt,
		}
		if row.IntervalSeconds != nil {
			wf.Interval = time.Duration(*row.IntervalSeconds) * time.Second
		}
		out = append(out, wf)
	}
	return out, nil
}

func (r *incidentScanRepository) ListAvailabilityEnabledWorkflows(ctx context.Context) ([]incident.EnabledWorkflow, error) {
	var rows []enabledWorkflowRow
	err := DBWithContext(ctx, r.db).Raw(`
SELECT w.user_id, w.id AS workflow_id, w.interval_seconds, s.last_run_at, w.created_at
FROM incident_policies p
JOIN workflows w ON w.id = p.workflow_id
JOIN workflow_state s ON s.workflow_id = w.id
WHERE p.kind = 'availability_low'
  AND p.enabled = TRUE
  AND w.deleted_at IS NULL
`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]incident.EnabledWorkflow, 0, len(rows))
	for _, row := range rows {
		wf := incident.EnabledWorkflow{
			UserID: row.UserID, WorkflowID: row.WorkflowID,
			LastRunAt: row.LastRunAt, CreatedAt: row.CreatedAt,
		}
		if row.IntervalSeconds != nil {
			wf.Interval = time.Duration(*row.IntervalSeconds) * time.Second
		}
		out = append(out, wf)
	}
	return out, nil
}

func (r *incidentScanRepository) RunSuccessRatio24h(ctx context.Context, workflowID uuid.UUID, now time.Time) (float64, int64, error) {
	since := now.UTC().Add(-24 * time.Hour)
	var row struct {
		Success int64 `gorm:"column:success"`
		Total   int64 `gorm:"column:total"`
	}
	err := DBWithContext(ctx, r.db).Raw(`
SELECT
  COUNT(*) FILTER (WHERE status = 'success')::bigint AS success,
  COUNT(*) FILTER (WHERE status IN ('success','failed','error'))::bigint AS total
FROM runs
WHERE workflow_id = @workflow_id
  AND finished_at IS NOT NULL
  AND finished_at >= @since
`, map[string]any{"workflow_id": workflowID, "since": since}).Scan(&row).Error
	if err != nil {
		return 0, 0, err
	}
	if row.Total == 0 {
		return 0, 0, nil
	}
	return 100.0 * float64(row.Success) / float64(row.Total), row.Total, nil
}
