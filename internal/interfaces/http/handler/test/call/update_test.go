package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestCallHandler_Update_Success(t *testing.T) {
	m := newMocks()
	body := validCallBody()
	body["name"] = "auth"
	resp := doJSON(t, m, true, http.MethodPut, callPath(), body)
	assertStatus(t, resp, http.StatusOK)

	var out presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testCallID.String() || out.Name != "auth" {
		t.Fatalf("unexpected body %+v", out)
	}
	cmd := m.update.got
	if cmd.UserID != testutil.TestUserID || cmd.WorkflowID != testWorkflowID ||
		cmd.CallID != testCallID || cmd.Definition.Name != "auth" {
		t.Fatalf("unexpected command %+v", cmd)
	}
}

func TestCallHandler_Update_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPut, callPath(), validCallBody())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_Update_InvalidIDs(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/bad/calls/"+testCallID.String(), validCallBody())
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCallHandler_Update_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPut, callPath(), `{"name":`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.update.calls != 0 {
		t.Fatal("update must not be called on invalid JSON")
	}
}

func TestCallHandler_Update_HandlerError_WrongOwner(t *testing.T) {
	m := newMocks()
	m.update.err = domainworkflow.ErrWrongOwner
	resp := doJSON(t, m, true, http.MethodPut, callPath(), validCallBody())
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Workflow not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestCallHandler_Update_HandlerError_CallNotFound(t *testing.T) {
	m := newMocks()
	m.update.err = domainworkflow.ErrCallNotFound
	resp := doJSON(t, m, true, http.MethodPut, callPath(), validCallBody())
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Call not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestCallHandler_Update_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.update.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPut, callPath(), validCallBody())
	assertStatus(t, resp, http.StatusInternalServerError)
}
