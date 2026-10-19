package workflowtest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var stopPath = "/workflows/" + testWorkflowID.String() + "/stop"

func TestWorkflowHandler_Stop_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, stopPath, nil)
	assertStatus(t, resp, http.StatusAccepted)

	if m.stop.calls != 1 || m.stop.got.UserID != testutil.TestUserID || m.stop.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected command %+v", m.stop.got)
	}
	var out presenter.WorkflowRunResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.RunID != testRunID.String() {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestWorkflowHandler_Stop_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, stopPath, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.stop.calls != 0 {
		t.Fatal("stop must not be called without a user")
	}
}

func TestWorkflowHandler_Stop_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows/not-a-uuid/stop", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Stop_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.stop.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPost, stopPath, nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestWorkflowHandler_Stop_HandlerError_NoRunInProgress(t *testing.T) {
	m := newMocks()
	m.stop.err = domainworkflow.ErrNoRunInProgress
	resp := doJSON(t, m, true, http.MethodPost, stopPath, nil)
	assertStatus(t, resp, http.StatusConflict)
	out := testutil.DecodeJSONMap(t, resp)
	if out["message"] != "No run is in progress" || out["code"] != "NO_RUN_IN_PROGRESS" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Stop_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.stop.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, stopPath, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to stop workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
