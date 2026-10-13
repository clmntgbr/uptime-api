package handler

import (
	"context"

	workflowcmd "go-api/internal/application/command/workflow"
	queryworkflow "go-api/internal/application/query/workflow"
	domainworkflow "go-api/internal/domain/workflow"
)

type workflowCreateHandler interface {
	Handle(ctx context.Context, cmd workflowcmd.CreateWorkflowCommand) (*domainworkflow.Workflow, error)
}

type workflowUpdateHandler interface {
	Handle(ctx context.Context, cmd workflowcmd.UpdateWorkflowCommand) (*domainworkflow.Workflow, error)
}

type workflowListHandler interface {
	Handle(ctx context.Context, q queryworkflow.ListWorkflowsQuery) ([]domainworkflow.ListView, int64, error)
}

type workflowGetHandler interface {
	Handle(ctx context.Context, q queryworkflow.GetWorkflowQuery) (*domainworkflow.View, error)
}

type workflowDeleteHandler interface {
	Handle(ctx context.Context, cmd workflowcmd.DeleteWorkflowCommand) error
}
