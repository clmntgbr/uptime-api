package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func TestCallHandler_List_Success(t *testing.T) {
	m := newMocks()
	m.list.views = []domainworkflow.CallListView{
		{ID: testCallID, Name: "login"},
		{ID: uuid.MustParse("01960000-0000-7000-8000-0000000000c2"), Name: "orders"},
	}
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusOK)

	if m.list.got.UserID != testutil.TestUserID || m.list.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.list.got)
	}
	var out []presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 2 || out[0].ID != testCallID.String() || out[0].Name != "login" {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestCallHandler_List_Empty(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out == nil || len(out) != 0 {
		t.Fatalf("empty list must be [], got %#v", out)
	}
}

func TestCallHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_List_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/not-a-uuid/calls", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCallHandler_List_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.list.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_List_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list calls" {
		t.Fatalf("unexpected body %v", out)
	}
}
