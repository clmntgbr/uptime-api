package variabletest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestVariableHandler_ListStatics_Success(t *testing.T) {
	m := newMocks()
	m.listStatics.out = []domainworkflow.StaticVariable{{ID: testVariableID, Name: "base_url", Value: "https://x"}}
	resp := doJSON(t, m, true, http.MethodGet, workflowVarsPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 1 || out[0].Name != "base_url" || out[0].ID != testVariableID.String() {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestVariableHandler_ListStatics_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, workflowVarsPath(), nil), http.StatusUnauthorized)
}

func TestVariableHandler_ListStatics_InvalidWorkflowID(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), true, http.MethodGet, "/workflows/bad/variables", nil), http.StatusBadRequest)
}

func TestVariableHandler_ListStatics_WorkflowNotFound(t *testing.T) {
	m := newMocks()
	m.listStatics.err = domainworkflow.ErrNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, workflowVarsPath(), nil), http.StatusNotFound)
}

func TestVariableHandler_GetStatic_Success(t *testing.T) {
	m := newMocks()
	m.getStatic.out = &domainworkflow.StaticVariable{ID: testVariableID, Name: "base_url", Value: "https://x"}
	resp := doJSON(t, m, true, http.MethodGet, workflowVarsPath()+"/base_url", nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testVariableID.String() || out.Name != "base_url" {
		t.Fatalf("unexpected %+v", out)
	}
	if m.getStatic.got.Name != "base_url" || m.getStatic.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.getStatic.got)
	}
}

func TestVariableHandler_GetStatic_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodGet, workflowVarsPath()+"/base_url", nil), http.StatusUnauthorized)
}

func TestVariableHandler_GetStatic_NotFound(t *testing.T) {
	m := newMocks()
	m.getStatic.err = domainworkflow.ErrVariableNotFound
	assertStatus(t, doJSON(t, m, true, http.MethodGet, workflowVarsPath()+"/missing", nil), http.StatusNotFound)
}

func TestVariableHandler_CreateStatic_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, workflowVarsPath(), map[string]any{"name": "base_url", "value": "https://x"})
	assertStatus(t, resp, http.StatusCreated)
	var out presenter.VariableResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testVariableID.String() || out.Name != "base_url" {
		t.Fatalf("unexpected %+v", out)
	}
	if m.createStatic.got.Name != "base_url" || m.createStatic.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected command %+v", m.createStatic.got)
	}
}

func TestVariableHandler_CreateStatic_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPost, workflowVarsPath(), map[string]any{"name": "a", "value": "b"}), http.StatusUnauthorized)
}

func TestVariableHandler_CreateStatic_InvalidBody(t *testing.T) {
	assertStatus(t, doRaw(t, newMocks(), http.MethodPost, workflowVarsPath(), `{}`), http.StatusBadRequest)
}

func TestVariableHandler_UpdateStatic_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, workflowVarsPath()+"/base_url", map[string]any{"value": "https://y"})
	assertStatus(t, resp, http.StatusOK)
	if m.updateStatic.got.Name != "base_url" || m.updateStatic.got.Value != "https://y" {
		t.Fatalf("unexpected command %+v", m.updateStatic.got)
	}
}

func TestVariableHandler_UpdateStatic_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodPut, workflowVarsPath()+"/base_url", map[string]any{"value": "x"}), http.StatusUnauthorized)
}

func TestVariableHandler_DeleteStatic_Success(t *testing.T) {
	m := newMocks()
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, workflowVarsPath()+"/base_url", nil), http.StatusNoContent)
}

func TestVariableHandler_DeleteStatic_Unauthorized(t *testing.T) {
	assertStatus(t, doJSON(t, newMocks(), false, http.MethodDelete, workflowVarsPath()+"/base_url", nil), http.StatusUnauthorized)
}

func TestVariableHandler_DeleteStatic_InUse(t *testing.T) {
	m := newMocks()
	m.deleteStatic.err = domainworkflow.ErrVariableInUse
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, workflowVarsPath()+"/base_url", nil), http.StatusConflict)
}

func TestVariableHandler_DeleteStatic_WrongOwner(t *testing.T) {
	m := newMocks()
	m.deleteStatic.err = domainworkflow.ErrWrongOwner
	assertStatus(t, doJSON(t, m, true, http.MethodDelete, workflowVarsPath()+"/base_url", nil), http.StatusNotFound)
}
