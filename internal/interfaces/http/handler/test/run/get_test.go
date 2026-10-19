package runtest

import (
	"net/http"
	"testing"

	queryrun "go-api/internal/application/query/run"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var getPath = "/workflows/" + testWorkflowID.String() + "/runs/" + testRunID.String()

func TestRunHandler_GetByID_Success(t *testing.T) {
	m := newMocks()
	m.get.view = sampleDetail()

	resp := doJSON(t, m, true, http.MethodGet, getPath)
	assertStatus(t, resp, http.StatusOK)

	if m.get.calls != 1 || m.get.got.UserID != testutil.TestUserID ||
		m.get.got.WorkflowID != testWorkflowID || m.get.got.RunID != testRunID {
		t.Fatalf("unexpected query %+v", m.get.got)
	}
	var out presenter.RunDetailResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testRunID.String() || out.WorkflowID != testWorkflowID.String() ||
		out.Status != "success" || len(out.Calls) != 1 || out.Calls[0].Name != "login" {
		t.Fatalf("unexpected body %+v", out)
	}
	if out.Variables["base_url"] != "https://example.com" {
		t.Fatalf("unexpected variables %+v", out.Variables)
	}
}

func TestRunHandler_GetByID_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, getPath)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.get.calls != 0 {
		t.Fatal("get must not be called without a user")
	}
}

func TestRunHandler_GetByID_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/not-a-uuid/runs/"+testRunID.String())
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestRunHandler_GetByID_InvalidRunID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/runs/not-a-uuid")
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid run id" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestRunHandler_GetByID_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.get.err = queryrun.ErrRunNotFound
	resp := doJSON(t, m, true, http.MethodGet, getPath)
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Run not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestRunHandler_GetByID_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.get.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, getPath)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to get run" {
		t.Fatalf("unexpected body %v", out)
	}
}
