package statistics

import (
	"context"
	"time"

	domainstatistics "go-api/internal/domain/statistics"

	"github.com/google/uuid"
)

const (
	dashboardWindow    = 24 * time.Hour
	callsHistoryMonths = 6
)

type GetDashboardQuery struct {
	UserID     uuid.UUID
	WorkflowID *uuid.UUID // optional; scopes runs/calls/workflows to one workflow
	Now        time.Time  // optional; zero uses time.Now UTC
}

type GetDashboardHandler struct {
	repo domainstatistics.ReadRepository
}

func NewGetDashboardHandler(repo domainstatistics.ReadRepository) *GetDashboardHandler {
	return &GetDashboardHandler{repo: repo}
}

func (h *GetDashboardHandler) Handle(ctx context.Context, q GetDashboardQuery) (domainstatistics.DashboardStatistics, error) {
	now := q.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	since := now.Add(-dashboardWindow)
	untilDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dailySince := untilDay.AddDate(0, -callsHistoryMonths, 0)

	result, err := h.repo.Dashboard(ctx, q.UserID, q.WorkflowID, since, dailySince, untilDay)
	if err != nil {
		return domainstatistics.DashboardStatistics{}, err
	}
	result.WindowHours = int(dashboardWindow / time.Hour)
	if result.Runs.Daily == nil {
		result.Runs.Daily = []domainstatistics.RunDayCounts{}
	}
	if result.Calls.Daily == nil {
		result.Calls.Daily = []domainstatistics.CallDayCounts{}
	}
	if result.Incidents == nil {
		result.Incidents = []domainstatistics.IncidentRange{}
	}
	if result.CallDurations == nil {
		result.CallDurations = []domainstatistics.CallDurationSeries{}
	}
	if result.Runs.Total > 0 {
		result.SuccessRate = float64(result.Runs.Success) / float64(result.Runs.Total)
	}
	return result, nil
}
