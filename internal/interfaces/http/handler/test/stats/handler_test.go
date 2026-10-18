package statstest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	querystats "go-api/internal/application/query/stats"
	domainstats "go-api/internal/domain/stats"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
)

type mockDashboardHandler struct {
	calls int
	got   querystats.GetDashboardQuery
	out   domainstats.DashboardStats
	err   error
}

func (m *mockDashboardHandler) Handle(_ context.Context, q querystats.GetDashboardQuery) (domainstats.DashboardStats, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return domainstats.DashboardStats{}, m.err
	}
	return m.out, nil
}

func mount(h *handler.StatsHandler, authenticated bool) *fiber.App {
	app := testutil.NewTestApp()
	if authenticated {
		app.Get("/stats", testutil.WithUserWithoutProject(testutil.TestUserID), h.GetDashboard)
	} else {
		app.Get("/stats", h.GetDashboard)
	}
	return app
}

func doGet(t *testing.T, app *fiber.App) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(http.MethodGet, "/stats", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	return resp
}

func TestStatsHandler_GetDashboard_Success(t *testing.T) {
	m := &mockDashboardHandler{out: domainstats.DashboardStats{
		WindowHours: 24,
		Workflows:   domainstats.WorkflowCounts{Total: 3, Active: 2, Disabled: 1},
		Runs: domainstats.RunCounts{
			Total: 10, Success: 7, Failed: 2, Error: 1,
			Daily: []domainstats.RunDayCounts{{
				Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				Total: 3, Success: 2, Failed: 1,
			}},
		},
		Calls: domainstats.CallCounts{
			Total: 20, Success: 15, Failed: 3, Error: 1, Skipped: 1,
			Daily: []domainstats.CallDayCounts{{
				Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				Total: 5, Success: 4, Failed: 1,
			}},
		},
		SuccessRate: 0.7,
	}}
	resp := doGet(t, mount(handler.NewStatsHandler(m), true))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if m.calls != 1 || m.got.UserID != testutil.TestUserID {
		t.Fatalf("unexpected query %+v calls=%d", m.got, m.calls)
	}
	var out presenter.DashboardStatsResponse
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

func TestStatsHandler_GetDashboard_Unauthorized(t *testing.T) {
	resp := doGet(t, mount(handler.NewStatsHandler(&mockDashboardHandler{}), false))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestStatsHandler_GetDashboard_HandlerError_Internal(t *testing.T) {
	m := &mockDashboardHandler{err: errors.New("db down")}
	resp := doGet(t, mount(handler.NewStatsHandler(m), true))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	body := testutil.DecodeJSONMap(t, resp)
	if body["message"] != "Failed to load dashboard stats" {
		t.Fatalf("unexpected body %v", body)
	}
}
