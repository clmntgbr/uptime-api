package workflowtest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var runPath = "/workflows/" + testWorkflowID.String() + "/runs"

func TestWorkflowHandler_Run_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusAccepted)

	if m.run.calls != 1 || m.run.got.UserID != testutil.TestUserID || m.run.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected command %+v", m.run.got)
	}
	var out presenter.WorkflowRunResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.RunID != testRunID.String() {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestWorkflowHandler_Run_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.run.calls != 0 {
		t.Fatal("run must not be called without a user")
	}
}

func TestWorkflowHandler_Run_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows/not-a-uuid/runs", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
	if m.run.calls != 0 {
		t.Fatal("run must not be called with an invalid id")
	}
}

func TestWorkflowHandler_Run_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.run.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Workflow not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Run_HandlerError_InProgress(t *testing.T) {
	m := newMocks()
	m.run.err = domainworkflow.ErrRunInProgress
	resp := doJSON(t, m, true, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusConflict)
	out := testutil.DecodeJSONMap(t, resp)
	if out["message"] != "A run is already in progress" || out["code"] != "RUN_IN_PROGRESS" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Run_HandlerError_NoCalls(t *testing.T) {
	m := newMocks()
	m.run.err = domainworkflow.ErrNoCalls
	resp := doJSON(t, m, true, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusBadRequest)
	out := testutil.DecodeJSONMap(t, resp)
	if out["message"] != "Workflow has no calls" || out["code"] != "NO_CALLS" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Run_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.run.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, runPath, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to run workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
