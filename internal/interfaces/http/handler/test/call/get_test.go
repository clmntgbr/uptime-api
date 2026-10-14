package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestCallHandler_GetByID_Success(t *testing.T) {
	m := newMocks()
	m.get.view = &domainworkflow.CallListView{ID: testCallID, Name: "login"}
	resp := doJSON(t, m, true, http.MethodGet, callPath(), nil)
	assertStatus(t, resp, http.StatusOK)

	if m.get.got.UserID != testutil.TestUserID || m.get.got.WorkflowID != testWorkflowID || m.get.got.CallID != testCallID {
		t.Fatalf("unexpected query %+v", m.get.got)
	}
	var out presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testCallID.String() || out.Name != "login" {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestCallHandler_GetByID_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, callPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_GetByID_InvalidIDs(t *testing.T) {
	cases := map[string]string{
		"bad workflow": "/workflows/not-a-uuid/calls/" + testCallID.String(),
		"bad call":     "/workflows/" + testWorkflowID.String() + "/calls/not-a-uuid",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			resp := doJSON(t, m, true, http.MethodGet, path, nil)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.get.calls != 0 {
				t.Fatal("get must not be called with invalid ids")
			}
		})
	}
}

func TestCallHandler_GetByID_HandlerError_NotFound(t *testing.T) {
	for name, err := range map[string]error{
		"workflow": domainworkflow.ErrNotFound,
		"call":     domainworkflow.ErrCallNotFound,
	} {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			m.get.err = err
			resp := doJSON(t, m, true, http.MethodGet, callPath(), nil)
			assertStatus(t, resp, http.StatusNotFound)
		})
	}
}

func TestCallHandler_GetByID_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.get.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, callPath(), nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to get call" {
		t.Fatalf("unexpected body %v", out)
	}
}
