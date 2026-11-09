package call

import (
	"context"
	"testing"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type stubWorkflows struct {
	w *domainworkflow.Workflow
}

func (s *stubWorkflows) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (s *stubWorkflows) Save(context.Context, *domainworkflow.Workflow) error { return nil }
func (s *stubWorkflows) Update(context.Context, *domainworkflow.Workflow) error {
	return nil
}
func (s *stubWorkflows) Delete(context.Context, uuid.UUID) error { return nil }
func (s *stubWorkflows) GetByID(context.Context, uuid.UUID) (*domainworkflow.Workflow, error) {
	return s.w, nil
}
func (s *stubWorkflows) ListByUser(context.Context, uuid.UUID) ([]*domainworkflow.Workflow, error) {
	return nil, nil
}

type stubGlobals struct {
	items []variable.Global
}

func (s *stubGlobals) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (s *stubGlobals) ListByUser(context.Context, uuid.UUID) ([]variable.Global, error) {
	return s.items, nil
}
func (s *stubGlobals) ListForContext(context.Context, uuid.UUID) ([]variable.Global, error) {
	return s.items, nil
}
func (s *stubGlobals) FindByID(context.Context, uuid.UUID, uuid.UUID) (*variable.Global, error) {
	return nil, nil
}
func (s *stubGlobals) FindByName(context.Context, uuid.UUID, string) (*variable.Global, error) {
	return nil, nil
}
func (s *stubGlobals) Names(context.Context, uuid.UUID) ([]string, error) { return nil, nil }
func (s *stubGlobals) Save(context.Context, *variable.Global) error       { return nil }
func (s *stubGlobals) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

type stubSuggestions struct {
	keys   []string
	values []string
}

func (s *stubSuggestions) SearchKeys(context.Context, uuid.UUID, domaincall.MapField, string, int) ([]string, error) {
	return append([]string(nil), s.keys...), nil
}
func (s *stubSuggestions) SearchValues(context.Context, uuid.UUID, domaincall.MapField, string, int) ([]string, error) {
	return append([]string(nil), s.values...), nil
}

func TestSuggestFieldsHandler_FiltersForeignWorkflowVars(t *testing.T) {
	userID := uuid.New()
	wfID := uuid.New()
	w := &domainworkflow.Workflow{
		ID: wfID, UserID: userID,
		Statics: []domainworkflow.StaticVariable{{Name: "api_key"}},
		Calls: []domaincall.Call{{
			Extractions: []domaincall.Extraction{{Name: "token"}},
		}},
	}
	h := NewSuggestFieldsHandler(
		&stubWorkflows{w: w},
		&stubGlobals{items: []variable.Global{{Name: "env"}}},
		&stubSuggestions{values: []string{
			"Bearer literal",
			"Bearer {{token}}",
			"key={{api_key}}",
			"{{env}}/v1",
			"{{timestamp}}",
			"Bearer {{other_wf_token}}",
			"{{missing}}",
		}},
	)
	got, err := h.Handle(context.Background(), SuggestFieldsQuery{
		UserID: userID, WorkflowID: wfID, Kind: SuggestHeaderValues, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Bearer literal",
		"Bearer {{token}}",
		"key={{api_key}}",
		"{{env}}/v1",
		"{{timestamp}}",
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}
