package handler

import (
	"context"

	cmdvariable "go-api/internal/application/command/variable"
	queryvariable "go-api/internal/application/query/variable"
	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
)

type variableCreateGlobalHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.CreateGlobalCommand) (*variable.Global, error)
}

type variableUpdateGlobalHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.UpdateGlobalCommand) (*variable.Global, error)
}

type variableDeleteGlobalHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.DeleteGlobalVariableCommand) error
}

type variableListGlobalsHandler interface {
	Handle(ctx context.Context, q queryvariable.ListGlobalsQuery) ([]variable.Global, error)
}

type variableGetGlobalHandler interface {
	Handle(ctx context.Context, q queryvariable.GetGlobalQuery) (*variable.Global, error)
}

type variableCreateStaticHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.CreateStaticCommand) (*domainworkflow.StaticVariable, error)
}

type variableUpdateStaticHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.UpdateStaticCommand) (*domainworkflow.StaticVariable, error)
}

type variableDeleteStaticHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.DeleteStaticCommand) error
}

type variableListStaticsHandler interface {
	Handle(ctx context.Context, q queryvariable.ListStaticsQuery) ([]domainworkflow.StaticVariable, error)
}

type variableGetStaticHandler interface {
	Handle(ctx context.Context, q queryvariable.GetStaticQuery) (*domainworkflow.StaticVariable, error)
}

type variableCreateCallHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.CreateCallVariableCommand) (*domaincall.Extraction, error)
}

type variableUpdateCallHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.UpdateCallVariableCommand) (*domaincall.Extraction, error)
}

type variableDeleteCallHandler interface {
	Handle(ctx context.Context, cmd cmdvariable.DeleteCallVariableCommand) error
}

type variableListCallHandler interface {
	Handle(ctx context.Context, q queryvariable.ListCallVariablesQuery) ([]domaincall.Extraction, error)
}

type variableGetCallHandler interface {
	Handle(ctx context.Context, q queryvariable.GetCallVariableQuery) (*domaincall.Extraction, error)
}

type variableListAvailableHandler interface {
	Handle(ctx context.Context, q queryvariable.ListAvailableQuery) ([]variable.Available, error)
}

type variableListResponseKeysHandler interface {
	Handle(ctx context.Context, q queryvariable.ListResponseKeysQuery) ([]string, error)
}
