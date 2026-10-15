package variabletest

import (
	"net/http"
	"testing"

	cmdvariable "go-api/internal/application/command/variable"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestVariableHandler_ListGlobals_Success(t *testing.T) {
	m := newMocks()
	m.listGlobals.out = []variable.Global{{ID: testVariableID, Name: "api_key", Value: "secret", Secret: true}}
	resp := doJSON(t, m, true, http.MethodGet, "/variables", nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 1 || out[0].ID != testVariableID.String() || out[0].Name != "api_key" || out[0].Value != variable.MaskedValue || !out[0].Secret {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestVariableHandler_ListGlobals_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, "/variables", nil), http.StatusUnauthorized)
}

func TestVariableHandler_ListGlobals_HandlerError(t *testing.T) {
	m := newMocks()
	m.listGlobals.err = errDatabase
	assertStatus(t, doJSON(t, m, true, http.MethodGet, "/variables", nil), http.StatusInternalServerError)
}

func TestVariableHandler_GetGlobal_Success(t *testing.T) {
	m := newMocks()
	m.getGlobal.out = &variable.Global{ID: testVariableID, Name: "base_url", Value: "https://x.test"}
	resp := doJSON(t, m, true, http.MethodGet, globalVarPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testVariableID.String() || out.Name != "base_url" || out.Value != "https://x.test" || m.getGlobal.got.ID != testVariableID {
		t.Fatalf("unexpected %+v cmd=%+v", out, m.getGlobal.got)
	}
}

func TestVariableHandler_GetGlobal_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, globalVarPath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_GetGlobal_InvalidID(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), true, http.MethodGet, "/variables/not-a-uuid", nil), http.StatusBadRequest)
}

func TestVariableHandler_GetGlobal_NotFound(t *testing.T) {
	m := newMocks()
	m.getGlobal.err = variable.ErrNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, globalVarPath(), nil), http.StatusNotFound)
}

func TestVariableHandler_CreateGlobal_Success(t *testing.T) {
	m := newMocks()
	body := map[string]any{"name": "token", "value": "abc", "secret": true}
	resp := doJSON(t, m, true, http.MethodPost, "/variables", body)
	assertStatus(t, resp, http.StatusCreated)
	if m.createGlobal.got.Name != "token" || m.createGlobal.got.Value != "abc" || !m.createGlobal.got.Secret {
		t.Fatalf("unexpected command %+v", m.createGlobal.got)
	}
}

func TestVariableHandler_CreateGlobal_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPost, "/variables", map[string]any{"name": "a", "value": "b"}), http.StatusUnauthorized)
}

func TestVariableHandler_CreateGlobal_InvalidBody(t *testing.T) {
	assertStatus(t, doRaw(t, newMocks(), http.MethodPost, "/variables", `[]`), http.StatusBadRequest)
}

func TestVariableHandler_CreateGlobal_Exists(t *testing.T) {
	m := newMocks()
	m.createGlobal.err = variable.ErrAlreadyExists
	assertStatus(t, doJSON(t, m, true, http.MethodPost, "/variables", map[string]any{"name": "token", "value": "x"}), http.StatusConflict)
}

func TestVariableHandler_CreateGlobal_NameConflict(t *testing.T) {
	m := newMocks()
	m.createGlobal.err = &domainworkflow.ValidationError{Variable: "token", Message: "name already used", Kind: domainworkflow.ErrNameConflict}
	assertStatus(t, doJSON(t, m, true, http.MethodPost, "/variables", map[string]any{"name": "token", "value": "x"}), http.StatusBadRequest)
}

func TestVariableHandler_CreateGlobal_InvalidName(t *testing.T) {
	m := newMocks()
	m.createGlobal.err = variable.ErrInvalidName
	assertStatus(t, doJSON(t, m, true, http.MethodPost, "/variables", map[string]any{"name": "token", "value": "x"}), http.StatusBadRequest)
}

func TestVariableHandler_UpdateGlobal_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, globalVarPath(), map[string]any{"value": "new", "secret": false})
	assertStatus(t, resp, http.StatusOK)
	if m.updateGlobal.got.ID != testVariableID || m.updateGlobal.got.Value != "new" {
		t.Fatalf("unexpected command %+v", m.updateGlobal.got)
	}
}

func TestVariableHandler_UpdateGlobal_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPut, globalVarPath(), map[string]any{"value": "x"}), http.StatusUnauthorized)
}

func TestVariableHandler_UpdateGlobal_NotFound(t *testing.T) {
	m := newMocks()
	m.updateGlobal.err = variable.ErrNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodPut, globalVarPath(), map[string]any{"value": "x"}), http.StatusNotFound)
}

func TestVariableHandler_DeleteGlobal_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, globalVarPath(), nil)
	assertStatus(t, resp, http.StatusNoContent)
	if m.deleteGlobal.got.ID != testVariableID {
		t.Fatalf("unexpected command %+v", m.deleteGlobal.got)
	}
}

func TestVariableHandler_DeleteGlobal_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodDelete, globalVarPath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_DeleteGlobal_InUse(t *testing.T) {
	m := newMocks()
	m.deleteGlobal.err = cmdvariable.ErrVariableInUse
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, globalVarPath(), nil), http.StatusConflict)
}

func TestVariableHandler_DeleteGlobal_Internal(t *testing.T) {
	m := newMocks()
	m.deleteGlobal.err = errDatabase
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, globalVarPath(), nil), http.StatusInternalServerError)
}
