package statisticstest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	querystatistics "go-api/internal/application/query/statistics"
	domainstatistics "go-api/internal/domain/statistics"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type mockDashboardHandler struct {
	calls int
	got   querystatistics.GetDashboardQuery
	out   domainstatistics.DashboardStatistics
	err   error
}

func (m *mockDashboardHandler) Handle(_ context.Context, q querystatistics.GetDashboardQuery) (domainstatistics.DashboardStatistics, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return domainstatistics.DashboardStatistics{}, m.err
	}
	return m.out, nil
}

func mount(h *handler.StatisticsHandler, authenticated bool) *fiber.App {
	app := testutil.NewTestApp()
	if authenticated {
		app.Get("/statistics", testutil.WithUserWithoutProject(testutil.TestUserID), h.GetDashboard)
	} else {
		app.Get("/statistics", h.GetDashboard)
	}
	return app
}

func doGet(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	if path == "" {
		path = "/statistics"
	}
	req, err := testutil.JSONRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	return resp
}

func TestStatisticsHandler_GetDashboard_Success(t *testing.T) {
	m := &mockDashboardHandler{out: domainstatistics.DashboardStatistics{
		WindowHours: 24,
		Workflows:   domainstatistics.WorkflowCounts{Total: 3, Active: 2, Disabled: 1},
		Runs: domainstatistics.RunCounts{
			Total: 10, Success: 7, Failed: 2, Error: 1,
			Daily: []domainstatistics.RunDayCounts{{
				Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				Total: 3, Success: 2, Failed: 1,
			}},
		},
		Calls: domainstatistics.CallCounts{
			Total: 20, Success: 15, Failed: 3, Error: 1, Skipped: 1,
			Daily: []domainstatistics.CallDayCounts{{
				Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				Total: 5, Success: 4, Failed: 1,
			}},
		},
		SuccessRate: 0.7,
	}}
	resp := doGet(t, mount(handler.NewStatisticsHandler(m), true), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if m.calls != 1 || m.got.UserID != testutil.TestUserID || m.got.WorkflowID != nil {
		t.Fatalf("unexpected query %+v calls=%d", m.got, m.calls)
	}
	var out presenter.DashboardStatisticsResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.WindowHours != 24 || out.Workflows.Active != 2 || out.Runs.Success != 7 ||
		out.Calls.Failed != 3 || out.SuccessRate != 0.7 {
		t.Fatalf("unexpected body %+v", out)
	}
	if len(out.Runs.Daily) != 1 || out.Runs.Daily[0].Date != "2026-10-15" || out.Runs.Daily[0].Total != 3 {
		t.Fatalf("unexpected runs daily %+v", out.Runs.Daily)
	}
	if len(out.Calls.Daily) != 1 || out.Calls.Daily[0].Date != "2026-10-15" || out.Calls.Daily[0].Total != 5 {
		t.Fatalf("unexpected calls daily %+v", out.Calls.Daily)
	}
}

func TestStatisticsHandler_GetDashboard_WithWorkflowID(t *testing.T) {
	m := &mockDashboardHandler{out: domainstatistics.DashboardStatistics{WindowHours: 24}}
	workflowID := uuid.MustParse("01960000-0000-7000-8000-000000000010")
	resp := doGet(t, mount(handler.NewStatisticsHandler(m), true), "/statistics?workflowId="+workflowID.String())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if m.got.WorkflowID == nil || *m.got.WorkflowID != workflowID {
		t.Fatalf("expected workflowId %s, got %+v", workflowID, m.got.WorkflowID)
	}
}

func TestStatisticsHandler_GetDashboard_InvalidWorkflowID(t *testing.T) {
	m := &mockDashboardHandler{}
	resp := doGet(t, mount(handler.NewStatisticsHandler(m), true), "/statistics?workflowId=not-a-uuid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if m.calls != 0 {
		t.Fatal("handler must not be called on invalid workflowId")
	}
	body := testutil.DecodeJSONMap(t, resp)
	if body["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestStatisticsHandler_GetDashboard_Unauthorized(t *testing.T) {
	resp := doGet(t, mount(handler.NewStatisticsHandler(&mockDashboardHandler{}), false), "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestStatisticsHandler_GetDashboard_HandlerError_Internal(t *testing.T) {
	m := &mockDashboardHandler{err: errors.New("db down")}
	resp := doGet(t, mount(handler.NewStatisticsHandler(m), true), "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	body := testutil.DecodeJSONMap(t, resp)
	if body["message"] != "Failed to load dashboard statistics" {
		t.Fatalf("unexpected body %v", body)
	}
}
