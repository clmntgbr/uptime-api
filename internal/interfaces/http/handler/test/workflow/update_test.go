package workflowtest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var updatePath = "/workflows/" + testWorkflowID.String()

func TestWorkflowHandler_Update_Success(t *testing.T) {
	m := newMocks()
	body := validWorkflowBody()
	body["name"] = "Checkout v2"
	resp := doJSON(t, m, true, http.MethodPut, updatePath, body)
	assertStatus(t, resp, http.StatusOK)

	var out presenter.WorkflowIDResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testWorkflowID.String() {
		t.Fatalf("response id: got %q", out.ID)
	}
	cmd := m.update.got
	if m.update.calls != 1 || cmd.UserID != testutil.TestUserID || cmd.WorkflowID != testWorkflowID ||
		cmd.Definition.Name != "Checkout v2" || len(cmd.Definition.Calls) != 0 {
		t.Fatalf("unexpected command %+v", cmd)
	}
}

func TestWorkflowHandler_Update_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPut, updatePath, validWorkflowBody())
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.update.calls != 0 {
		t.Fatal("update must not be called without a user")
	}
}

func TestWorkflowHandler_Update_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/not-a-uuid", validWorkflowBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
	if m.update.calls != 0 {
		t.Fatal("update must not be called with an invalid id")
	}
}

func TestWorkflowHandler_Update_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPut, updatePath, `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.update.calls != 0 {
		t.Fatal("update must not be called on an invalid body")
	}
}

func TestWorkflowHandler_Update_ValidationFailed(t *testing.T) {
	m := newMocks()
	body := validWorkflowBody()
	body["intervalSeconds"] = 30
	resp := doJSON(t, m, true, http.MethodPut, updatePath, body)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["errors"] == nil {
		t.Fatalf("expected field errors, got %v", out)
	}
	if m.update.calls != 0 {
		t.Fatal("update must not be called on an invalid payload")
	}
}

func TestWorkflowHandler_Update_HandlerError_NotFound(t *testing.T) {
	for name, err := range map[string]error{
		"not found":   domainworkflow.ErrNotFound,
		"wrong owner": domainworkflow.ErrWrongOwner,
	} {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			m.update.err = err
			resp := doJSON(t, m, true, http.MethodPut, updatePath, validWorkflowBody())
			assertStatus(t, resp, http.StatusNotFound)
			if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Workflow not found" {
				t.Fatalf("another user's workflow must look missing, got %v", out)
			}
		})
	}
}

func TestWorkflowHandler_Update_HandlerError_InvalidWorkflow(t *testing.T) {
	m := newMocks()
	m.update.err = &domainworkflow.ValidationError{Message: "interval must be a whole number of minutes, at least 1 minute"}
	resp := doJSON(t, m, true, http.MethodPut, updatePath, validWorkflowBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["code"] != "INVALID_WORKFLOW" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Update_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.update.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPut, updatePath, validWorkflowBody())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to update workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
