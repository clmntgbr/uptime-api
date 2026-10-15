package variabletest

import (
	"net/http"
	"testing"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestVariableHandler_ListCallVariables_Success(t *testing.T) {
	m := newMocks()
	m.listCall.out = []domaincall.Extraction{{ID: testVariableID, Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token"}}
	resp := doJSON(t, m, true, http.MethodGet, callVarsPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 1 || out[0].Path != "$.token" || out[0].ID != testVariableID.String() {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestVariableHandler_ListCallVariables_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, callVarsPath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_ListCallVariables_InvalidCallID(t *testing.T) {
	path := "/workflows/" + testWorkflowID.String() + "/calls/bad/variables"
	assertStatus(t, doJSON(t, newMocks(), true, http.MethodGet, path, nil), http.StatusBadRequest)
}

func TestVariableHandler_ListCallVariables_CallNotFound(t *testing.T) {
	m := newMocks()
	m.listCall.err = domainworkflow.ErrCallNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, callVarsPath(), nil), http.StatusNotFound)
}

func TestVariableHandler_GetCallVariable_Success(t *testing.T) {
	m := newMocks()
	m.getCall.out = &domaincall.Extraction{ID: testVariableID, Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token"}
	resp := doJSON(t, m, true, http.MethodGet, callVarsPath()+"/token", nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testVariableID.String() || out.Name != "token" {
		t.Fatalf("unexpected %+v", out)
	}
	if m.getCall.got.Name != "token" || m.getCall.got.CallID != testCallID {
		t.Fatalf("unexpected query %+v", m.getCall.got)
	}
}

func TestVariableHandler_GetCallVariable_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, callVarsPath()+"/token", nil), http.StatusUnauthorized)
}

func TestVariableHandler_GetCallVariable_NotFound(t *testing.T) {
	m := newMocks()
	m.getCall.err = domainworkflow.ErrVariableNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, callVarsPath()+"/missing", nil), http.StatusNotFound)
}

func TestVariableHandler_CreateCallVariable_Success(t *testing.T) {
	m := newMocks()
	body := map[string]any{"name": "token", "path": "$.access_token", "source": "json"}
	resp := doJSON(t, m, true, http.MethodPost, callVarsPath(), body)
	assertStatus(t, resp, http.StatusCreated)
	var out presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testVariableID.String() || out.Name != "token" {
		t.Fatalf("unexpected %+v", out)
	}
	if m.createCall.got.Name != "token" || m.createCall.got.Path != "$.access_token" || m.createCall.got.Source != "json" {
		t.Fatalf("unexpected command %+v", m.createCall.got)
	}
}

func TestVariableHandler_CreateCallVariable_WithoutPath(t *testing.T) {
	m := newMocks()
	m.createCall.out = &domaincall.Extraction{Name: "code", Source: domaincall.ExtractStatus}
	resp := doJSON(t, m, true, http.MethodPost, callVarsPath(), map[string]any{"name": "code"})
	assertStatus(t, resp, http.StatusCreated)
	if m.createCall.got.Name != "code" || m.createCall.got.Path != "" {
		t.Fatalf("unexpected command %+v", m.createCall.got)
	}
}

func TestVariableHandler_CreateCallVariable_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPost, callVarsPath(), map[string]any{"name": "a"}), http.StatusUnauthorized)
}

func TestVariableHandler_CreateCallVariable_Exists(t *testing.T) {
	m := newMocks()
	m.createCall.err = variable.ErrAlreadyExists
	assertStatus(t, doJSON(t, m, true, http.MethodPost, callVarsPath(), map[string]any{"name": "token", "path": "$.t"}), http.StatusConflict)
}

func TestVariableHandler_UpdateCallVariable_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, callVarsPath()+"/token", map[string]any{"path": "$.new", "source": "json"})
	assertStatus(t, resp, http.StatusOK)
	if m.updateCall.got.Name != "token" || m.updateCall.got.Path != "$.new" {
		t.Fatalf("unexpected command %+v", m.updateCall.got)
	}
}

func TestVariableHandler_UpdateCallVariable_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPut, callVarsPath()+"/token", map[string]any{"path": "$.x"}), http.StatusUnauthorized)
}

func TestVariableHandler_DeleteCallVariable_Success(t *testing.T) {
	m := newMocks()
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, callVarsPath()+"/token", nil), http.StatusNoContent)
}

func TestVariableHandler_DeleteCallVariable_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodDelete, callVarsPath()+"/token", nil), http.StatusUnauthorized)
}

func TestVariableHandler_DeleteCallVariable_InUse(t *testing.T) {
	m := newMocks()
	m.deleteCall.err = domainworkflow.ErrVariableInUse
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, callVarsPath()+"/token", nil), http.StatusConflict)
}

func TestVariableHandler_ListResponseKeys_Success(t *testing.T) {
	m := newMocks()
	m.listResponseKeys.out = []string{"access_token", "user", "user.id", "items[0].id"}
	resp := doJSON(t, m, true, http.MethodGet, responseKeysPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []string
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 4 || out[0] != "access_token" || m.listResponseKeys.got.CallID != testCallID {
		t.Fatalf("unexpected %+v query=%+v", out, m.listResponseKeys.got)
	}
}

func TestVariableHandler_ListResponseKeys_Empty(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, responseKeysPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []string
	testutil.DecodeJSON(t, resp, &out)
	if out == nil || len(out) != 0 {
		t.Fatalf("expected empty array, got %#v", out)
	}
}

func TestVariableHandler_ListResponseKeys_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, responseKeysPath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_ListResponseKeys_CallNotFound(t *testing.T) {
	m := newMocks()
	m.listResponseKeys.err = domainworkflow.ErrCallNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, responseKeysPath(), nil), http.StatusNotFound)
}

func TestVariableHandler_ListResponseKeys_InvalidCallID(t *testing.T) {
	path := "/workflows/" + testWorkflowID.String() + "/calls/bad/response-keys"
	assertStatus(t, doJSON(t, newMocks(), true, http.MethodGet, path, nil), http.StatusBadRequest)
}

func TestVariableHandler_ListResponseKeys_HandlerError(t *testing.T) {
	m := newMocks()
	m.listResponseKeys.err = errDatabase
	assertStatus(t, doJSON(t, m, true, http.MethodGet, responseKeysPath(), nil), http.StatusInternalServerError)
}

func TestVariableHandler_ListAvailable_Success(t *testing.T) {
	m := newMocks()
	m.listAvailable.out = []variable.Available{
		{Name: "api_key", Type: variable.TypeGlobal, Secret: true, Value: "raw"},
		{Name: "base_url", Type: variable.TypeStatic, Value: "https://x"},
		{Name: "token", Type: variable.TypeCall, Path: "$.token", Source: "json", CallID: testCallID, CallName: "login"},
	}
	resp := doJSON(t, m, true, http.MethodGet, availablePath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 3 || out[0].Value != variable.MaskedValue || out[2].CallID != testCallID.String() {
		t.Fatalf("unexpected %+v", out)
	}
	if m.listAvailable.got.CallID != testCallID || m.listAvailable.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.listAvailable.got)
	}
}

func TestVariableHandler_ListAvailable_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, availablePath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_ListAvailable_HandlerError(t *testing.T) {
	m := newMocks()
	m.listAvailable.err = errDatabase
	assertStatus(t, doJSON(t, m, true, http.MethodGet, availablePath(), nil), http.StatusInternalServerError)
}
