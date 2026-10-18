package stats

import (
	"context"
	"time"

	domainstats "go-api/internal/domain/stats"

	"github.com/google/uuid"
)

const (
	dashboardWindow     = 24 * time.Hour
	callsHistoryMonths  = 6
)

type GetDashboardQuery struct {
	UserID uuid.UUID
	Now    time.Time // optional; zero uses time.Now UTC
}

type GetDashboardHandler struct {
	repo domainstats.ReadRepository
}

func NewGetDashboardHandler(repo domainstats.ReadRepository) *GetDashboardHandler {
	return &GetDashboardHandler{repo: repo}
}

func (h *GetDashboardHandler) Handle(ctx context.Context, q GetDashboardQuery) (domainstats.DashboardStats, error) {
	now := q.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	since := now.Add(-dashboardWindow)
	untilDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dailySince := untilDay.AddDate(0, -callsHistoryMonths, 0)

	stats, err := h.repo.Dashboard(ctx, q.UserID, since, dailySince, untilDay)
	if err != nil {
		return domainstats.DashboardStats{}, err
	}
	stats.WindowHours = int(dashboardWindow / time.Hour)
	if stats.Runs.Daily == nil {
		stats.Runs.Daily = []domainstats.RunDayCounts{}
	}
	if stats.Calls.Daily == nil {
		stats.Calls.Daily = []domainstats.CallDayCounts{}
	}
	if stats.Runs.Total > 0 {
		stats.SuccessRate = float64(stats.Runs.Success) / float64(stats.Runs.Total)
	}
	return stats, nil
}
