package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/testutil"
)

func TestCallHandler_Delete_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, callPath(), nil)
	assertStatus(t, resp, http.StatusNoContent)
	if m.delete.calls != 1 || m.delete.got.UserID != testutil.TestUserID ||
		m.delete.got.WorkflowID != testWorkflowID || m.delete.got.CallID != testCallID {
		t.Fatalf("unexpected command %+v", m.delete.got)
	}
}

func TestCallHandler_Delete_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodDelete, callPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_Delete_InvalidCallID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, callsPath()+"/not-a-uuid", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCallHandler_Delete_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.delete.err = domainworkflow.ErrCallNotFound
	resp := doJSON(t, m, true, http.MethodDelete, callPath(), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_Delete_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.delete.err = errDatabase
	resp := doJSON(t, m, true, http.MethodDelete, callPath(), nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to delete call" {
		t.Fatalf("unexpected body %v", out)
	}
}
