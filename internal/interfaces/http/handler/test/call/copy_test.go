package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestCallHandler_Copy_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusCreated)

	if m.copy.calls != 1 || m.copy.got.UserID != testutil.TestUserID ||
		m.copy.got.WorkflowID != testWorkflowID || m.copy.got.CallID != testCallID {
		t.Fatalf("unexpected command %+v calls=%d", m.copy.got, m.copy.calls)
	}
	var out presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Name != "login" || out.Method != "POST" || out.Position != 2 {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestCallHandler_Copy_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_Copy_InvalidIDs(t *testing.T) {
	m := newMocks()
	cases := map[string]string{
		"bad workflow": "/workflows/not-a-uuid/calls/" + testCallID.String() + "/copy",
		"bad call":     "/workflows/" + testWorkflowID.String() + "/calls/not-a-uuid/copy",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			resp := doJSON(t, m, true, http.MethodPost, path, nil)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.copy.calls != 0 {
				t.Fatalf("handler must not be called")
			}
		})
	}
}

func TestCallHandler_Copy_HandlerError_CallNotFound(t *testing.T) {
	m := newMocks()
	m.copy.err = domainworkflow.ErrCallNotFound
	resp := doJSON(t, m, true, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_Copy_HandlerError_WorkflowNotFound(t *testing.T) {
	m := newMocks()
	m.copy.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_Copy_HandlerError_InvalidWorkflow(t *testing.T) {
	m := newMocks()
	m.copy.err = &domainworkflow.ValidationError{Message: "a workflow has at most 20 calls"}
	resp := doJSON(t, m, true, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["code"] != "INVALID_WORKFLOW" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestCallHandler_Copy_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.copy.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, callPath()+"/copy", nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to copy call" {
		t.Fatalf("unexpected body %v", out)
	}
}
