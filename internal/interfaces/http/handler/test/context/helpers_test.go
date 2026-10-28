package contexttest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	querycontext "go-api/internal/application/query/contextpanel"
	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000e1")
	testRunID      = uuid.MustParse("01960000-0000-7000-8000-0000000000e2")
	testCallID     = uuid.MustParse("01960000-0000-7000-8000-0000000000e3")
	errDatabase    = errors.New("pq: connection refused")
)

type mockLatestRunHandler struct {
	calls int
	got   querycontext.GetLatestRunQuery
	view  *domainrun.LatestView
	err   error
}

func (m *mockLatestRunHandler) Handle(_ context.Context, q querycontext.GetLatestRunQuery) (*domainrun.LatestView, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.view, nil
}

type mockVariablesHandler struct {
	calls int
	got   querycontext.ListVariablesQuery
	views []domainworkflow.ContextVariableView
	err   error
}

func (m *mockVariablesHandler) Handle(_ context.Context, q querycontext.ListVariablesQuery) ([]domainworkflow.ContextVariableView, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.views, nil
}

type mocks struct {
	latest    *mockLatestRunHandler
	variables *mockVariablesHandler
}

func newMocks() mocks {
	return mocks{
		latest:    &mockLatestRunHandler{},
		variables: &mockVariablesHandler{},
	}
}

func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewContextHandler(m.latest, m.variables)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows/:id/context/runs/latest", h.GetLatestRun)
	app.Get("/workflows/:id/context/variables", h.ListVariables)
	return app
}

func latestPath() string {
	return "/workflows/" + testWorkflowID.String() + "/context/runs/latest"
}

func variablesPath() string {
	return "/workflows/" + testWorkflowID.String() + "/context/variables"
}

func doGET(t *testing.T, m mocks, authenticated bool, path string) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := mountRoutes(m, authenticated).Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status: got %d want %d", resp.StatusCode, want)
	}
}
