package read

import (
	"context"
	"math"
	"time"

	domainstatistics "go-api/internal/domain/statistics"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type statisticsRepository struct {
	db *gorm.DB
}

func NewStatisticsRepository(db *gorm.DB) domainstatistics.ReadRepository {
	return &statisticsRepository{db: db}
}

const workflowCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE enabled)::bigint AS active,
    COUNT(*) FILTER (WHERE NOT enabled)::bigint AS disabled
FROM workflows
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (NOT @filter_workflow OR id = @workflow_id)`

const runCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE status = 'success')::bigint AS success,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed,
    COUNT(*) FILTER (WHERE status = 'error')::bigint AS error,
    COUNT(*) FILTER (WHERE status = 'skipped')::bigint AS skipped,
    COUNT(*) FILTER (WHERE status = 'running')::bigint AS running
FROM runs
WHERE user_id = @user_id AND started_at >= @since
  AND (NOT @filter_workflow OR workflow_id = @workflow_id)`

const callCountsSQL = `
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE cr.status = 'success')::bigint AS success,
    COUNT(*) FILTER (WHERE cr.status = 'failed')::bigint AS failed,
    COUNT(*) FILTER (WHERE cr.status = 'error')::bigint AS error,
    COUNT(*) FILTER (WHERE cr.status = 'skipped')::bigint AS skipped
FROM call_runs cr
JOIN runs r ON r.id = cr.run_id
WHERE r.user_id = @user_id AND r.started_at >= @since
  AND (NOT @filter_workflow OR r.workflow_id = @workflow_id)`

const latencyP95SQL = `
SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) AS latency_p95_ms
FROM runs
WHERE user_id = @user_id AND started_at >= @since
  AND status IN ('success', 'failed', 'error')
  AND (NOT @filter_workflow OR workflow_id = @workflow_id)`

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
counts AS (
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
      AND (NOT @filter_workflow OR workflow_id = @workflow_id)
    GROUP BY 1
),
latency AS (
    SELECT
        (started_at AT TIME ZONE 'UTC')::date AS day,
        percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) AS latency_p95_ms
    FROM runs
    WHERE user_id = @user_id
      AND started_at >= @daily_since
      AND started_at < (CAST(@until_day AS date) + INTERVAL '1 day')
      AND status IN ('success', 'failed', 'error')
      AND (NOT @filter_workflow OR workflow_id = @workflow_id)
    GROUP BY 1
)
SELECT
    d.day AS date,
    COALESCE(c.total, 0)::bigint AS total,
    COALESCE(c.success, 0)::bigint AS success,
    COALESCE(c.failed, 0)::bigint AS failed,
    COALESCE(c.error, 0)::bigint AS error,
    COALESCE(c.skipped, 0)::bigint AS skipped,
    COALESCE(c.running, 0)::bigint AS running,
    l.latency_p95_ms AS latency_p95_ms
FROM days d
LEFT JOIN counts c ON c.day = d.day
LEFT JOIN latency l ON l.day = d.day
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
      AND (NOT @filter_workflow OR r.workflow_id = @workflow_id)
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

// incidentsSQL collapses contiguous failed/error runs into [start, end] ranges.
// End is NULL when the island includes the chronologically last finished run.
const incidentsSQL = `
WITH finished AS (
    SELECT
        id,
        started_at,
        COALESCE(finished_at, started_at) AS ended_at,
        CASE WHEN status IN ('failed', 'error') THEN 1 ELSE 0 END AS is_bad,
        ROW_NUMBER() OVER (ORDER BY started_at ASC, id ASC) AS rn,
        ROW_NUMBER() OVER (
            PARTITION BY (status IN ('failed', 'error'))
            ORDER BY started_at ASC, id ASC
        ) AS rn_status
    FROM runs
    WHERE user_id = @user_id
      AND started_at >= @daily_since
      AND started_at < (CAST(@until_day AS date) + INTERVAL '1 day')
      AND status IN ('success', 'failed', 'error')
      AND (NOT @filter_workflow OR workflow_id = @workflow_id)
),
bad_islands AS (
    SELECT
        (rn - rn_status) AS island,
        MIN(started_at) AS start_at,
        MAX(ended_at) AS end_at,
        MAX(rn) AS max_rn
    FROM finished
    WHERE is_bad = 1
    GROUP BY 1
),
last_rn AS (
    SELECT MAX(rn) AS rn FROM finished
)
SELECT
    b.start_at AS start,
    CASE WHEN b.max_rn = l.rn THEN NULL ELSE b.end_at END AS "end"
FROM bad_islands b
CROSS JOIN last_rn l
ORDER BY b.start_at`

type runCountsRow struct {
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	Running int64
}

type runDayRow struct {
	Date         time.Time `gorm:"column:date"`
	Total        int64
	Success      int64
	Failed       int64
	Error        int64
	Skipped      int64
	Running      int64
	LatencyP95Ms *float64  `gorm:"column:latency_p95_ms"`
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

type latencyRow struct {
	LatencyP95Ms *float64 `gorm:"column:latency_p95_ms"`
}

type incidentRow struct {
	Start time.Time  `gorm:"column:start"`
	End   *time.Time `gorm:"column:end"`
}

func roundMs(v float64) int64 {
	return int64(math.Round(v))
}

func ptrMs(v *float64) *int64 {
	if v == nil {
		return nil
	}
	ms := roundMs(*v)
	return &ms
}

func (r *statisticsRepository) Dashboard(
	ctx context.Context,
	userID uuid.UUID,
	workflowID *uuid.UUID,
	since, dailySince, untilDay time.Time,
) (domainstatistics.DashboardStatistics, error) {
	filterWorkflow := false
	workflowParam := uuid.Nil
	if workflowID != nil {
		filterWorkflow = true
		workflowParam = *workflowID
	}
	params := map[string]any{
		"user_id":         userID,
		"since":           since.UTC(),
		"daily_since":     dailySince.UTC(),
		"until_day":       untilDay.UTC(),
		"filter_workflow": filterWorkflow,
		"workflow_id":     workflowParam,
	}
	db := r.db.WithContext(ctx)

	var workflows domainstatistics.WorkflowCounts
	if err := db.Raw(workflowCountsSQL, params).Scan(&workflows).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}

	var runTotals runCountsRow
	if err := db.Raw(runCountsSQL, params).Scan(&runTotals).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}

	var latency latencyRow
	if err := db.Raw(latencyP95SQL, params).Scan(&latency).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}

	var runRows []runDayRow
	if err := db.Raw(runDailySQL, params).Scan(&runRows).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}
	runDaily := make([]domainstatistics.RunDayCounts, 0, len(runRows))
	for _, row := range runRows {
		runDaily = append(runDaily, domainstatistics.RunDayCounts{
			Date:         row.Date,
			Total:        row.Total,
			Success:      row.Success,
			Failed:       row.Failed,
			Error:        row.Error,
			Skipped:      row.Skipped,
			Running:      row.Running,
			LatencyP95Ms: ptrMs(row.LatencyP95Ms),
		})
	}

	var callTotals callCountsRow
	if err := db.Raw(callCountsSQL, params).Scan(&callTotals).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}

	var callRows []callDayRow
	if err := db.Raw(callDailySQL, params).Scan(&callRows).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}
	callDaily := make([]domainstatistics.CallDayCounts, 0, len(callRows))
	for _, row := range callRows {
		callDaily = append(callDaily, domainstatistics.CallDayCounts{
			Date:    row.Date,
			Total:   row.Total,
			Success: row.Success,
			Failed:  row.Failed,
			Error:   row.Error,
			Skipped: row.Skipped,
		})
	}

	var incidentRows []incidentRow
	if err := db.Raw(incidentsSQL, params).Scan(&incidentRows).Error; err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}
	incidents := make([]domainstatistics.IncidentRange, 0, len(incidentRows))
	openIncident := false
	for _, row := range incidentRows {
		inc := domainstatistics.IncidentRange{Start: row.Start, End: row.End}
		if inc.End == nil {
			openIncident = true
		}
		incidents = append(incidents, inc)
	}

	return domainstatistics.DashboardStatistics{
		Workflows:    workflows,
		LatencyP95Ms: ptrMs(latency.LatencyP95Ms),
		OpenIncident: openIncident,
		Incidents:    incidents,
		Runs: domainstatistics.RunCounts{
			Total: runTotals.Total, Success: runTotals.Success, Failed: runTotals.Failed,
			Error: runTotals.Error, Skipped: runTotals.Skipped, Running: runTotals.Running,
			Daily: runDaily,
		},
		Calls: domainstatistics.CallCounts{
			Total: callTotals.Total, Success: callTotals.Success, Failed: callTotals.Failed,
			Error: callTotals.Error, Skipped: callTotals.Skipped, Daily: callDaily,
		},
	}, nil
}
