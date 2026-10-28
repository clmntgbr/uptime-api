package contexttest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestContextHandler_ListVariables_Success(t *testing.T) {
	token := "eyJhbGci"
	m := newMocks()
	m.variables.views = []domainworkflow.ContextVariableView{
		{Scope: domainworkflow.ContextScopeGlobal, Name: "base_url", Value: strPtr("example.com"), Secret: false},
		{Scope: domainworkflow.ContextScopeStatic, Name: "api_key", Secret: true},
		{
			Scope: domainworkflow.ContextScopeWorkflow, Name: "token", Value: &token, Secret: false,
			ProducedBy: &domainworkflow.ContextProducerView{CallID: testCallID, Name: "Login call"},
			Status:     domainworkflow.ContextVarStatusOK,
		},
		{
			Scope: domainworkflow.ContextScopeWorkflow, Name: "first_id", Secret: false,
			ProducedBy: &domainworkflow.ContextProducerView{CallID: testCallID, Name: "Orders call"},
			Status:     domainworkflow.ContextVarStatusMissing,
		},
	}
	resp := doGET(t, m, true, variablesPath())
	assertStatus(t, resp, http.StatusOK)
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control: %q", resp.Header.Get("Cache-Control"))
	}
	if m.variables.calls != 1 || m.variables.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.variables.got)
	}
	var out presenter.ContextVariablesResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Variables) != 4 ||
		out.Variables[0].Scope != "global" || out.Variables[0].Value == nil ||
		out.Variables[1].Secret != true || out.Variables[1].Value != nil ||
		out.Variables[2].Status != "ok" || out.Variables[2].ProducedBy == nil ||
		out.Variables[2].ProducedBy.CallID != testCallID.String() ||
		out.Variables[3].Status != "missing" || out.Variables[3].Value != nil {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestContextHandler_ListVariables_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doGET(t, m, false, variablesPath())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestContextHandler_ListVariables_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doGET(t, m, true, "/workflows/not-a-uuid/context/variables")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestContextHandler_ListVariables_NotFound(t *testing.T) {
	m := newMocks()
	m.variables.err = domainworkflow.ErrNotFound
	resp := doGET(t, m, true, variablesPath())
	assertStatus(t, resp, http.StatusNotFound)
}

func TestContextHandler_ListVariables_Internal(t *testing.T) {
	m := newMocks()
	m.variables.err = errDatabase
	resp := doGET(t, m, true, variablesPath())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list context variables" {
		t.Fatalf("unexpected body %v", out)
	}
}

func strPtr(s string) *string { return &s }
