package statistics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DashboardStatistics is the read model for the account dashboard.
type DashboardStatistics struct {
	WindowHours  int
	Workflows    WorkflowCounts
	Runs         RunCounts
	Calls        CallCounts
	SuccessRate  float64
	LatencyP95Ms *int64
	// CallDurations is the mean wall-clock duration of each call, per UTC day.
	// Stacking a day rebuilds the average run.
	CallDurations []CallDurationSeries
}

type WorkflowCounts struct {
	Total    int64
	Active   int64
	Disabled int64
}

type RunCounts struct {
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	Running int64
	// Daily is day-by-day run volume over the history window (zero-filled).
	Daily []RunDayCounts
}

// RunDayCounts is one calendar day of run aggregates (UTC).
type RunDayCounts struct {
	Date    time.Time
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	Running int64
}

type CallCounts struct {
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
	// Daily is day-by-day call volume over the history window (zero-filled).
	Daily []CallDayCounts
}

// CallDayCounts is one calendar day of call_run aggregates (UTC).
type CallDayCounts struct {
	Date         time.Time
	Total        int64
	Success      int64
	Failed       int64
	Error        int64
	Skipped      int64
	LatencyP95Ms *int64
}

// CallDurationSeries is one call's mean duration over the history window.
type CallDurationSeries struct {
	ID       string
	Name     string
	Position int
	Daily    []CallDurationPoint
}

// CallDurationPoint is the mean timings.total for one call on one UTC day.
type CallDurationPoint struct {
	Date       time.Time
	DurationMs int64
	Samples    int64
}

// ReadRepository loads aggregated dashboard counters for a user.
type ReadRepository interface {
	// Dashboard loads 24h counters (since) and runs/calls daily series
	// from dailySince through untilDay (inclusive, UTC dates).
	// When workflowID is set, all aggregates are scoped to that workflow.
	Dashboard(ctx context.Context, userID uuid.UUID, workflowID *uuid.UUID, since, dailySince, untilDay time.Time) (DashboardStatistics, error)
}
