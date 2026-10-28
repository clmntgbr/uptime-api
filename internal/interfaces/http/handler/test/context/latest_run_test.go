package contexttest

import (
	"net/http"
	"testing"
	"time"

	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestContextHandler_GetLatestRun_Success(t *testing.T) {
	httpStatus := 200
	m := newMocks()
	m.latest.view = &domainrun.LatestView{
		ID: testRunID, Status: domainrun.StatusSuccess,
		StartedAt: time.Date(2026, 10, 28, 22, 40, 0, 0, time.UTC), DurationMS: 392,
		Calls: []domainrun.LatestCallView{{
			CallID: &testCallID, Name: "Login call", Position: 1,
			Status: domainrun.CallSuccess, OffsetMS: 0, DurationMS: 90, HTTPStatus: &httpStatus,
		}},
	}
	resp := doGET(t, m, true, latestPath())
	assertStatus(t, resp, http.StatusOK)
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control: %q", resp.Header.Get("Cache-Control"))
	}
	if m.latest.calls != 1 || m.latest.got.UserID != testutil.TestUserID ||
		m.latest.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.latest.got)
	}
	var out presenter.ContextLatestRunResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Run == nil || out.Run.ID != testRunID.String() || out.Run.Status != "success" ||
		out.Run.DurationMs != 392 || len(out.Run.Calls) != 1 ||
		out.Run.Calls[0].Name != "Login call" || out.Run.Calls[0].OffsetMs != 0 ||
		out.Run.Calls[0].HTTPStatus == nil || *out.Run.Calls[0].HTTPStatus != 200 {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestContextHandler_GetLatestRun_NullWhenEmpty(t *testing.T) {
	m := newMocks()
	resp := doGET(t, m, true, latestPath())
	assertStatus(t, resp, http.StatusOK)
	var out presenter.ContextLatestRunResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Run != nil {
		t.Fatalf("expected run null, got %+v", out.Run)
	}
}

func TestContextHandler_GetLatestRun_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doGET(t, m, false, latestPath())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestContextHandler_GetLatestRun_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doGET(t, m, true, "/workflows/not-a-uuid/context/runs/latest")
	assertStatus(t, resp, http.StatusBadRequest)
	if m.latest.calls != 0 {
		t.Fatalf("handler must not be called")
	}
}

func TestContextHandler_GetLatestRun_NotFound(t *testing.T) {
	m := newMocks()
	m.latest.err = domainworkflow.ErrNotFound
	resp := doGET(t, m, true, latestPath())
	assertStatus(t, resp, http.StatusNotFound)
}

func TestContextHandler_GetLatestRun_Internal(t *testing.T) {
	m := newMocks()
	m.latest.err = errDatabase
	resp := doGET(t, m, true, latestPath())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to get latest run" {
		t.Fatalf("unexpected body %v", out)
	}
}
