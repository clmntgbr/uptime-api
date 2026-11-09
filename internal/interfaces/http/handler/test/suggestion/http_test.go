package suggestiontest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	querycall "go-api/internal/application/query/call"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000d1")

type mockSuggest struct {
	got  querycall.SuggestFieldsQuery
	out  []string
	err  error
	calls int
}

func (m *mockSuggest) Handle(_ context.Context, q querycall.SuggestFieldsQuery) ([]string, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.out, nil
}

func mount(t *testing.T, suggest *mockSuggest, withUser bool) *fiber.App {
	t.Helper()
	app := testutil.NewTestApp()
	h := handler.NewSuggestionHandler(suggest)
	var group fiber.Router = app.Group("/api")
	if withUser {
		group = app.Group("/api", testutil.WithActiveProject(testutil.TestUserID, testutil.TestProjectID))
	}
	group.Get("/workflows/:id/suggestions/header-keys", h.HeaderKeys)
	group.Get("/workflows/:id/suggestions/header-values", h.HeaderValues)
	group.Get("/workflows/:id/suggestions/query-keys", h.QueryKeys)
	group.Get("/workflows/:id/suggestions/query-values", h.QueryValues)
	return app
}

func doGet(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestSuggestionHandler_HeaderKeys_Success(t *testing.T) {
	m := &mockSuggest{out: []string{"Authorization", "Accept"}}
	resp := doGet(t, mount(t, m, true),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/header-keys?q=Auth&limit=10")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if m.got.Kind != querycall.SuggestHeaderKeys || m.got.Search != "Auth" || m.got.Limit != 10 {
		t.Fatalf("got %+v", m.got)
	}
	var out []string
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 2 || out[0] != "Authorization" {
		t.Fatalf("%#v", out)
	}
}

func TestSuggestionHandler_QueryValues_Success(t *testing.T) {
	m := &mockSuggest{out: []string{"1", "{{page}}"}}
	resp := doGet(t, mount(t, m, true),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/query-values")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if m.got.Kind != querycall.SuggestQueryValues {
		t.Fatalf("kind %s", m.got.Kind)
	}
}

func TestSuggestionHandler_HeaderValues_Unauthorized(t *testing.T) {
	resp := doGet(t, mount(t, &mockSuggest{}, false),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/header-values")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSuggestionHandler_QueryKeys_NotFound(t *testing.T) {
	m := &mockSuggest{err: domainworkflow.ErrNotFound}
	resp := doGet(t, mount(t, m, true),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/query-keys")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSuggestionHandler_HeaderKeys_BadWorkflowID(t *testing.T) {
	resp := doGet(t, mount(t, &mockSuggest{}, true), "/api/workflows/bad/suggestions/header-keys")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSuggestionHandler_HeaderKeys_BadLimit(t *testing.T) {
	resp := doGet(t, mount(t, &mockSuggest{}, true),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/header-keys?limit=nope")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSuggestionHandler_HeaderKeys_InternalError(t *testing.T) {
	m := &mockSuggest{err: errors.New("db down")}
	resp := doGet(t, mount(t, m, true),
		"/api/workflows/"+testWorkflowID.String()+"/suggestions/header-keys")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSuggestionHandler_AllKinds(t *testing.T) {
	cases := []struct {
		path string
		kind querycall.SuggestKind
	}{
		{"/suggestions/header-keys", querycall.SuggestHeaderKeys},
		{"/suggestions/header-values", querycall.SuggestHeaderValues},
		{"/suggestions/query-keys", querycall.SuggestQueryKeys},
		{"/suggestions/query-values", querycall.SuggestQueryValues},
	}
	for _, tc := range cases {
		m := &mockSuggest{out: []string{}}
		resp := doGet(t, mount(t, m, true), "/api/workflows/"+testWorkflowID.String()+tc.path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d", tc.path, resp.StatusCode)
		}
		if m.got.Kind != tc.kind {
			t.Fatalf("%s kind got %s want %s", tc.path, m.got.Kind, tc.kind)
		}
	}
}
