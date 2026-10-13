package workflowtest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/testutil"
)

var deletePath = "/workflows/" + testWorkflowID.String()

func TestWorkflowHandler_Delete_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, deletePath, nil)
	assertStatus(t, resp, http.StatusNoContent)
	if m.delete.calls != 1 || m.delete.got.UserID != testutil.TestUserID || m.delete.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected command %+v", m.delete.got)
	}
}

func TestWorkflowHandler_Delete_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodDelete, deletePath, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.delete.calls != 0 {
		t.Fatal("delete must not be called without a user")
	}
}

func TestWorkflowHandler_Delete_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, "/workflows/not-a-uuid", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
	if m.delete.calls != 0 {
		t.Fatal("delete must not be called with an invalid id")
	}
}

func TestWorkflowHandler_Delete_HandlerError_NotFound(t *testing.T) {
	for name, err := range map[string]error{
		"not found":   domainworkflow.ErrNotFound,
		"wrong owner": domainworkflow.ErrWrongOwner,
	} {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			m.delete.err = err
			resp := doJSON(t, m, true, http.MethodDelete, deletePath, nil)
			assertStatus(t, resp, http.StatusNotFound)
			if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Workflow not found" {
				t.Fatalf("another user's workflow must look missing, got %v", out)
			}
		})
	}
}

func TestWorkflowHandler_Delete_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.delete.err = errDatabase
	resp := doJSON(t, m, true, http.MethodDelete, deletePath, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to delete workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
