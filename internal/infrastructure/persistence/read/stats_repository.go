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

const daysCTE = `
WITH days AS (
    SELECT generate_series(
        CAST(@daily_since AS date),
        CAST(@until_day AS date),
        INTERVAL '1 day'
    )::date AS day
)`

// runDailySQL returns one row per UTC calendar day from daily_since through
// until_day (inclusive), left-joined onto run aggregates so missing days are zero.
const runDailySQL = daysCTE + `
,
agg AS (
    SELECT
        (started_at AT TIME ZONE 'UTC')::date AS day,
        COUNT(*)::bigint AS total,
        COUNT(*) FILTER (WHERE status = 'success')::bigint AS success,
        COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed,
        COUNT(*) FILTER (WHERE status = 'error')::bigint AS error,
        COUNT(*) FILTER (WHERE status = 'skipped')::bigint AS skipped,
        COUNT(*) FILTER (WHERE status = 'running')::bigint AS running
    FROM runs
    WHERE user_id = @user_id
      AND started_at >= @daily_since
      AND started_at < (CAST(@until_day AS date) + INTERVAL '1 day')
    GROUP BY 1
)
SELECT
    d.day AS date,
    COALESCE(a.total, 0)::bigint AS total,
    COALESCE(a.success, 0)::bigint AS success,
    COALESCE(a.failed, 0)::bigint AS failed,
    COALESCE(a.error, 0)::bigint AS error,
    COALESCE(a.skipped, 0)::bigint AS skipped,
    COALESCE(a.running, 0)::bigint AS running
FROM days d
LEFT JOIN agg a ON a.day = d.day
ORDER BY d.day`

// callDailySQL returns one row per UTC calendar day from daily_since through
// until_day (inclusive), left-joined onto call_run aggregates so missing days
// are zero. Prefer call_runs.started_at; fall back to the parent run start.
const callDailySQL = daysCTE + `
,
agg AS (
    SELECT
        (COALESCE(cr.started_at, r.started_at) AT TIME ZONE 'UTC')::date AS day,
        COUNT(*)::bigint AS total,
        COUNT(*) FILTER (WHERE cr.status = 'success')::bigint AS success,
        COUNT(*) FILTER (WHERE cr.status = 'failed')::bigint AS failed,
        COUNT(*) FILTER (WHERE cr.status = 'error')::bigint AS error,
        COUNT(*) FILTER (WHERE cr.status = 'skipped')::bigint AS skipped
    FROM call_runs cr
    JOIN runs r ON r.id = cr.run_id
    WHERE r.user_id = @user_id
      AND COALESCE(cr.started_at, r.started_at) >= @daily_since
      AND COALESCE(cr.started_at, r.started_at) < (CAST(@until_day AS date) + INTERVAL '1 day')
    GROUP BY 1
)
SELECT
    d.day AS date,
    COALESCE(a.total, 0)::bigint AS total,
    COALESCE(a.success, 0)::bigint AS success,
    COALESCE(a.failed, 0)::bigint AS failed,
    COALESCE(a.error, 0)::bigint AS error,
    COALESCE(a.skipped, 0)::bigint AS skipped
FROM days d
LEFT JOIN agg a ON a.day = d.day
ORDER BY d.day`

type runCountsRow struct {
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	Running int64
}

type runDayRow struct {
	Date    time.Time `gorm:"column:date"`
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	Running int64
}

type callCountsRow struct {
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
}

type callDayRow struct {
	Date    time.Time `gorm:"column:date"`
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
}

func (r *statsRepository) Dashboard(
	ctx context.Context,
	userID uuid.UUID,
	since, dailySince, untilDay time.Time,
) (domainstats.DashboardStats, error) {
	params := map[string]any{
		"user_id":     userID,
		"since":       since.UTC(),
		"daily_since": dailySince.UTC(),
		"until_day":   untilDay.UTC(),
	}
	db := r.db.WithContext(ctx)

	var workflows domainstats.WorkflowCounts
	if err := db.Raw(workflowCountsSQL, params).Scan(&workflows).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	var runTotals runCountsRow
	if err := db.Raw(runCountsSQL, params).Scan(&runTotals).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	var runRows []runDayRow
	if err := db.Raw(runDailySQL, params).Scan(&runRows).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}
	runDaily := make([]domainstats.RunDayCounts, 0, len(runRows))
	for _, row := range runRows {
		runDaily = append(runDaily, domainstats.RunDayCounts{
			Date:    row.Date,
			Total:   row.Total,
			Success: row.Success,
			Failed:  row.Failed,
			Error:   row.Error,
			Skipped: row.Skipped,
			Running: row.Running,
		})
	}

	var callTotals callCountsRow
	if err := db.Raw(callCountsSQL, params).Scan(&callTotals).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}

	var callRows []callDayRow
	if err := db.Raw(callDailySQL, params).Scan(&callRows).Error; err != nil {
		return domainstats.DashboardStats{}, err
	}
	callDaily := make([]domainstats.CallDayCounts, 0, len(callRows))
	for _, row := range callRows {
		callDaily = append(callDaily, domainstats.CallDayCounts{
			Date:    row.Date,
			Total:   row.Total,
			Success: row.Success,
			Failed:  row.Failed,
			Error:   row.Error,
			Skipped: row.Skipped,
		})
	}

	return domainstats.DashboardStats{
		Workflows: workflows,
		Runs: domainstats.RunCounts{
			Total: runTotals.Total, Success: runTotals.Success, Failed: runTotals.Failed,
			Error: runTotals.Error, Skipped: runTotals.Skipped, Running: runTotals.Running,
			Daily: runDaily,
		},
		Calls: domainstats.CallCounts{
			Total: callTotals.Total, Success: callTotals.Success, Failed: callTotals.Failed,
			Error: callTotals.Error, Skipped: callTotals.Skipped, Daily: callDaily,
		},
	}, nil
}
