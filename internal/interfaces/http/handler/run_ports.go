package handler

import (
	"context"

	queryrun "go-api/internal/application/query/run"
	domainrun "go-api/internal/domain/run"
)

type runListHandler interface {
	Handle(ctx context.Context, q queryrun.ListRunsQuery) ([]domainrun.View, int64, error)
}

type runGetHandler interface {
	Handle(ctx context.Context, q queryrun.GetRunQuery) (*domainrun.DetailView, error)
}
