package call

import (
	"context"
	"fmt"
	"strings"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type SuggestKind string

const (
	SuggestHeaderKeys   SuggestKind = "header-keys"
	SuggestHeaderValues SuggestKind = "header-values"
	SuggestQueryKeys    SuggestKind = "query-keys"
	SuggestQueryValues  SuggestKind = "query-values"
)

type SuggestFieldsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Kind       SuggestKind
	Search     string
	Limit      int
}

type SuggestFieldsHandler struct {
	workflows   domainworkflow.WriteRepository
	globals     variable.GlobalRepository
	suggestions domaincall.SuggestionsRepository
}

func NewSuggestFieldsHandler(
	workflows domainworkflow.WriteRepository,
	globals variable.GlobalRepository,
	suggestions domaincall.SuggestionsRepository,
) *SuggestFieldsHandler {
	return &SuggestFieldsHandler{
		workflows: workflows, globals: globals, suggestions: suggestions,
	}
}

func (h *SuggestFieldsHandler) Handle(ctx context.Context, q SuggestFieldsQuery) ([]string, error) {
	field, wantValues, err := parseSuggestKind(q.Kind)
	if err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = domaincall.DefaultSuggestionLimit
	}
	if limit > domaincall.MaxSuggestionLimit {
		limit = domaincall.MaxSuggestionLimit
	}
	search := strings.TrimSpace(q.Search)
	if len(search) > 200 {
		search = search[:200]
	}

	w, err := h.workflows.GetByID(ctx, q.WorkflowID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, domainworkflow.ErrNotFound
	}
	if w.UserID != q.UserID {
		return nil, domainworkflow.ErrWrongOwner
	}

	allowed, err := h.allowedNames(ctx, q.UserID, w)
	if err != nil {
		return nil, err
	}

	// Over-fetch values so foreign-workflow templates can be filtered out.
	fetchLimit := limit
	if wantValues {
		fetchLimit = limit * 5
		if fetchLimit > 200 {
			fetchLimit = 200
		}
	}

	var candidates []string
	if wantValues {
		candidates, err = h.suggestions.SearchValues(ctx, q.UserID, field, search, fetchLimit)
	} else {
		candidates, err = h.suggestions.SearchKeys(ctx, q.UserID, field, search, fetchLimit)
	}
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, limit)
	for _, item := range candidates {
		if !variable.AllReferencesAllowed(item, allowed) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (h *SuggestFieldsHandler) allowedNames(
	ctx context.Context, userID uuid.UUID, w *domainworkflow.Workflow,
) (map[string]struct{}, error) {
	allowed := make(map[string]struct{})
	for _, name := range variable.Builtins() {
		allowed[name] = struct{}{}
	}
	globals, err := h.globals.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	for _, g := range globals {
		allowed[g.Name] = struct{}{}
	}
	for _, s := range w.Statics {
		allowed[s.Name] = struct{}{}
	}
	for _, c := range w.Calls {
		for _, e := range c.Extractions {
			allowed[e.Name] = struct{}{}
		}
	}
	return allowed, nil
}

func parseSuggestKind(kind SuggestKind) (domaincall.MapField, bool, error) {
	switch kind {
	case SuggestHeaderKeys:
		return domaincall.MapHeaders, false, nil
	case SuggestHeaderValues:
		return domaincall.MapHeaders, true, nil
	case SuggestQueryKeys:
		return domaincall.MapQuery, false, nil
	case SuggestQueryValues:
		return domaincall.MapQuery, true, nil
	default:
		return "", false, fmt.Errorf("unknown suggestion kind %q", kind)
	}
}
