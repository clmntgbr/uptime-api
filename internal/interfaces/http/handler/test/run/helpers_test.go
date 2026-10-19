package runtest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	queryrun "go-api/internal/application/query/run"
	domainrun "go-api/internal/domain/run"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000d1")
	testRunID      = uuid.MustParse("01960000-0000-7000-8000-0000000000d2")
	errDatabase    = errors.New("pq: connection refused")
)

type mockListHandler struct {
	calls int
	got   queryrun.ListRunsQuery
	views []domainrun.View
	total int64
	err   error
}

func (m *mockListHandler) Handle(_ context.Context, q queryrun.ListRunsQuery) ([]domainrun.View, int64, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.views, m.total, nil
}

type mockGetHandler struct {
	calls int
	got   queryrun.GetRunQuery
	view  *domainrun.DetailView
	err   error
}

func (m *mockGetHandler) Handle(_ context.Context, q queryrun.GetRunQuery) (*domainrun.DetailView, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.view, nil
}

type mocks struct {
	list *mockListHandler
	get  *mockGetHandler
}

func newMocks() mocks {
	return mocks{list: &mockListHandler{}, get: &mockGetHandler{}}
}

func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewRunHandler(m.list, m.get)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows/:id/runs", h.List)
	app.Get("/workflows/:id/runs/:runId", h.GetByID)
	return app
}

func doJSON(t *testing.T, m mocks, authenticated bool, method, path string) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(method, path, nil)
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

func sampleListViews() []domainrun.View {
	return []domainrun.View{{
		ID: testRunID, WorkflowID: testWorkflowID, Trigger: "manual", Status: domainrun.StatusSuccess,
		StartedAt: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 10, 19, 10, 0, 1, 0, time.UTC),
		DurationMS: 1000,
		Counts:     domainrun.Counts{Success: 2},
	}}
}

func sampleDetail() *domainrun.DetailView {
	callID := uuid.MustParse("01960000-0000-7000-8000-0000000000d3")
	started := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 10, 19, 10, 0, 1, 0, time.UTC)
	return &domainrun.DetailView{
		View: domainrun.View{
			ID: testRunID, WorkflowID: testWorkflowID, Trigger: "manual", Status: domainrun.StatusSuccess,
			StartedAt: started, FinishedAt: finished, DurationMS: 1000,
			Counts: domainrun.Counts{Success: 1},
		},
		Variables: map[string]string{"base_url": "https://example.com"},
		Calls: []domainrun.CallRunView{{
			ID: uuid.MustParse("01960000-0000-7000-8000-0000000000d4"),
			CallID: &callID, Name: "login", Position: 1, Status: domainrun.CallSuccess,
			HTTPStatus: 200, StartedAt: &started, FinishedAt: &finished,
			Rules: []domainrun.RuleRun{},
		}},
	}
}
