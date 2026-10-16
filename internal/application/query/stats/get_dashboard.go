package stats

import (
	"context"
	"time"

	domainstats "go-api/internal/domain/stats"

	"github.com/google/uuid"
)

const dashboardWindow = 24 * time.Hour

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
	}
	since := now.Add(-dashboardWindow)
	stats, err := h.repo.Dashboard(ctx, q.UserID, since)
	if err != nil {
		return domainstats.DashboardStats{}, err
	}
	stats.WindowHours = int(dashboardWindow / time.Hour)
	if stats.Runs.Total > 0 {
		stats.SuccessRate = float64(stats.Runs.Success) / float64(stats.Runs.Total)
	}
	return stats, nil
}
