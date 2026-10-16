package handler

import (
	"context"

	querystats "go-api/internal/application/query/stats"
	domainstats "go-api/internal/domain/stats"
)

type statsDashboardHandler interface {
	Handle(ctx context.Context, q querystats.GetDashboardQuery) (domainstats.DashboardStats, error)
}
