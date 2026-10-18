package handler

import (
	"context"

	querystatistics "go-api/internal/application/query/statistics"
	domainstatistics "go-api/internal/domain/statistics"
)

type statisticsDashboardHandler interface {
	Handle(ctx context.Context, q querystatistics.GetDashboardQuery) (domainstatistics.DashboardStatistics, error)
}
