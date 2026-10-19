package workflowtest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var getPath = "/workflows/" + testWorkflowID.String()

func TestWorkflowHandler_GetByID_Success(t *testing.T) {
	m := newMocks()
	m.get.view = sampleView()

	resp := doJSON(t, m, true, http.MethodGet, getPath, nil)
	assertStatus(t, resp, http.StatusOK)

	if m.get.calls != 1 || m.get.got.UserID != testutil.TestUserID || m.get.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.get.got)
	}
	var out presenter.WorkflowResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testWorkflowID.String() || out.Name != "Checkout" || out.ScheduleType != "interval" || out.IntervalSeconds == nil || *out.IntervalSeconds != 300 || !out.Enabled || out.IsRunning {
		t.Fatalf("unexpected body %+v", out)
	}
	if out.NextRunAt == nil || *out.NextRunAt != "2026-10-13T12:05:00Z" {
		t.Fatalf("expected nextRunAt, got %+v", out.NextRunAt)
	}
	if out.LastRunAt == nil || *out.LastRunAt != "2026-10-13T12:00:00Z" {
		t.Fatalf("expected lastRunAt, got %+v", out.LastRunAt)
	}
	if out.LastStatus == nil || *out.LastStatus != "success" {
		t.Fatalf("expected lastStatus, got %+v", out.LastStatus)
	}
}

func TestWorkflowHandler_GetByID_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, getPath, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.get.calls != 0 {
		t.Fatal("get must not be called without a user")
	}
}

func TestWorkflowHandler_GetByID_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/not-a-uuid", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
	if m.get.calls != 0 {
		t.Fatal("get must not be called with an invalid id")
	}
}

func TestWorkflowHandler_GetByID_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.get.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, getPath, nil)
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Workflow not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_GetByID_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.get.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, getPath, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to get workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
