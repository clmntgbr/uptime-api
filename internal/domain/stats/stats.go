package stats

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DashboardStats is the read model for the account dashboard.
type DashboardStats struct {
	WindowHours int
	Workflows   WorkflowCounts
	Runs        RunCounts
	Calls       CallCounts
	SuccessRate float64
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
	Date    time.Time
	Total   int64
	Success int64
	Failed  int64
	Error   int64
	Skipped int64
}

// ReadRepository loads aggregated dashboard counters for a user.
type ReadRepository interface {
	// Dashboard loads 24h counters (since) and the calls daily series
	// from dailySince through untilDay (inclusive, UTC dates).
	Dashboard(ctx context.Context, userID uuid.UUID, since, dailySince, untilDay time.Time) (DashboardStats, error)
}
