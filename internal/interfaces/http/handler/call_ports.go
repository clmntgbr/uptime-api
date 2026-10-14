package handler

import (
	"context"

	callcmd "go-api/internal/application/command/call"
	querycall "go-api/internal/application/query/call"
	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"
)

type callCreateHandler interface {
	Handle(ctx context.Context, cmd callcmd.CreateCallCommand) (*domaincall.Call, error)
}

type callUpdateHandler interface {
	Handle(ctx context.Context, cmd callcmd.UpdateCallCommand) (*domaincall.Call, error)
}

type callDeleteHandler interface {
	Handle(ctx context.Context, cmd callcmd.DeleteCallCommand) error
}

type callListHandler interface {
	Handle(ctx context.Context, q querycall.ListCallsQuery) ([]domainworkflow.CallListView, error)
}

type callGetHandler interface {
	Handle(ctx context.Context, q querycall.GetCallQuery) (*domainworkflow.CallView, error)
}
