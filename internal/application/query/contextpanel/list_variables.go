package contextpanel

import (
	"context"
	"unicode/utf8"

	domainrun "go-api/internal/domain/run"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListVariablesQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type ListVariablesHandler struct {
	workflows domainworkflow.ReadRepository
	globals   variable.GlobalRepository
	runs      domainrun.ReadRepository
}

func NewListVariablesHandler(
	workflows domainworkflow.ReadRepository,
	globals variable.GlobalRepository,
	runs domainrun.ReadRepository,
) *ListVariablesHandler {
	return &ListVariablesHandler{workflows: workflows, globals: globals, runs: runs}
}

func (h *ListVariablesHandler) Handle(ctx context.Context, q ListVariablesQuery) ([]domainworkflow.ContextVariableView, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if wf == nil {
		return nil, domainworkflow.ErrNotFound
	}

	globals, err := h.globals.ListForContext(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	statics, err := h.workflows.ListContextStatics(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	extractions, err := h.workflows.ListContextExtractions(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	extracted, err := h.runs.FindLatestExtractedByWorkflow(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, err
	}

	out := make([]domainworkflow.ContextVariableView, 0, len(globals)+len(statics)+len(extractions))
	for _, g := range globals {
		out = append(out, contextValue(domainworkflow.ContextScopeGlobal, g.Name, g.Value, g.Secret, nil, ""))
	}
	for _, s := range statics {
		out = append(out, contextValue(domainworkflow.ContextScopeStatic, s.Name, s.Value, s.Secret, nil, ""))
	}
	for _, e := range extractions {
		producer := &domainworkflow.ContextProducerView{CallID: e.CallID, Name: e.CallName}
		raw := ""
		status := domainworkflow.ContextVarStatusMissing
		if values, ok := extracted[e.CallID]; ok {
			if v, ok := values[e.Name]; ok {
				raw = v
				status = domainworkflow.ContextVarStatusOK
			}
		}
		out = append(out, contextValue(
			domainworkflow.ContextScopeWorkflow, e.Name, raw, e.Secret, producer, status,
		))
	}
	return out, nil
}

func contextValue(
	scope, name, raw string,
	secret bool,
	producer *domainworkflow.ContextProducerView,
	status string,
) domainworkflow.ContextVariableView {
	view := domainworkflow.ContextVariableView{
		Scope:      scope,
		Name:       name,
		Secret:     secret,
		ProducedBy: producer,
		Status:     status,
	}
	if secret {
		return view
	}
	value, truncated := truncateContextValue(raw)
	view.Value = &value
	view.Truncated = truncated
	if status == domainworkflow.ContextVarStatusMissing {
		view.Value = nil
	}
	return view
}

func truncateContextValue(s string) (string, bool) {
	if utf8.RuneCountInString(s) <= domainworkflow.ContextValueMaxChars {
		return s, false
	}
	runes := []rune(s)
	return string(runes[:domainworkflow.ContextValueMaxChars]), true
}
