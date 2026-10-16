package read

import (
	"context"
	"time"

	domainstats "go-api/internal/domain/stats"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type statsRepository struct {
	db *gorm.DB
}

func NewStatsRepository(db *gorm.DB) domainstats.ReadRepository {
	return &statsRepository{db: db}
}

const workflowCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE enabled)::bigint AS active,
    COUNT(*) FILTER (WHERE NOT enabled)::bigint AS disabled
FROM workflows
WHERE user_id = @user_id AND deleted_at IS NULL`

const runCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE status = 'success')::bigint AS success,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed,
    COUNT(*) FILTER (WHERE status = 'error')::bigint AS error,
    COUNT(*) FILTER (WHERE status = 'skipped')::bigint AS skipped,
    COUNT(*) FILTER (WHERE status = 'running')::bigint AS running
FROM runs
WHERE user_id = @user_id AND started_at >= @since`

const callCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE cr.status = 'success')::bigint AS success,
    COUNT(*) FILTER (WHERE cr.status = 'failed')::bigint AS failed,
    COUNT(*) FILTER (WHERE cr.status = 'error')::bigint AS error,
    COUNT(*) FILTER (WHERE cr.status = 'skipped')::bigint AS skipped
FROM call_runs cr
JOIN runs r ON r.id = cr.run_id
WHERE r.user_id = @user_id AND r.started_at >= @since`

func (r *statsRepository) Dashboard(ctx context.Context, userID uuid.UUID, since time.Time) (domainstats.DashboardStats, error) {
	params := map[string]any{"user_id": userID, "since": since.UTC()}
	db := r.db.WithContext(ctx)

	var workflows domainstats.WorkflowCounts
	if err := db.Raw(workflowCountsSQL, params).Scan(&workflows).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	var runs domainstats.RunCounts
	if err := db.Raw(runCountsSQL, params).Scan(&runs).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	var calls domainstats.CallCounts
	if err := db.Raw(callCountsSQL, params).Scan(&calls).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	return domainstats.DashboardStats{
		Workflows: workflows,
		Runs:      runs,
		Calls:     calls,
	}, nil
}
