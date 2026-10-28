package handler

import (
	"context"

	querycontext "go-api/internal/application/query/contextpanel"
	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"
)

type contextLatestRunHandler interface {
	Handle(ctx context.Context, q querycontext.GetLatestRunQuery) (*domainrun.LatestView, error)
}

type contextVariablesHandler interface {
	Handle(ctx context.Context, q querycontext.ListVariablesQuery) ([]domainworkflow.ContextVariableView, error)
}
