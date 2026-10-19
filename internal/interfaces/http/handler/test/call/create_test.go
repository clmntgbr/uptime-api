package calltest

import (
	"net/http"
	"testing"
	"time"

	domainrule "go-api/internal/domain/rule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestCallHandler_Create_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, callsPath(), validCallBody())
	assertStatus(t, resp, http.StatusCreated)

	var out presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testCallID.String() || out.Name != "login" || out.Method != "POST" {
		t.Fatalf("unexpected body %+v", out)
	}
	cmd := m.create.got
	if cmd.UserID != testutil.TestUserID || cmd.WorkflowID != testWorkflowID ||
		cmd.Definition.Name != "login" || cmd.Definition.Method != "POST" ||
		cmd.Definition.Timeout != 5*time.Second || len(cmd.Definition.Extractions) != 0 ||
		len(cmd.Definition.Rules) != 1 || cmd.Definition.Rules[0].Subject != domainrule.SubjectStatus {
		t.Fatalf("unexpected command %+v", cmd)
	}
}

func TestCallHandler_Create_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, callsPath(), validCallBody())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_Create_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows/not-a-uuid/calls", validCallBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if m.create.calls != 0 {
		t.Fatal("create must not be called with an invalid workflow id")
	}
}

func TestCallHandler_Create_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, callsPath(), `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCallHandler_Create_ValidationFailed(t *testing.T) {
	cases := map[string]func(map[string]any){
		"missing name": func(b map[string]any) { b["name"] = "" },
		"missing url":  func(b map[string]any) { delete(b, "url") },
		"bad method":   func(b map[string]any) { b["method"] = "TRACE" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			body := validCallBody()
			mutate(body)
			resp := doJSON(t, m, true, http.MethodPost, callsPath(), body)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.create.calls != 0 {
				t.Fatal("create must not be called on invalid payload")
			}
		})
	}
}

func TestCallHandler_Create_InvalidRule(t *testing.T) {
	m := newMocks()
	body := validCallBody()
	body["rules"] = []map[string]any{
		{"subject": "status", "operator": "exceeds", "value": "200"},
	}
	resp := doJSON(t, m, true, http.MethodPost, callsPath(), body)
	assertStatus(t, resp, http.StatusBadRequest)
	out := testutil.DecodeJSONMap(t, resp)
	errs, _ := out["errors"].(map[string]any)
	if _, ok := errs["rules[0]"]; !ok {
		t.Fatalf("expected rules[0] error, got %v", out)
	}
}

func TestCallHandler_Create_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.create.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPost, callsPath(), validCallBody())
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_Create_HandlerError_InvalidWorkflow(t *testing.T) {
	m := newMocks()
	m.create.err = &domainworkflow.ValidationError{Call: "login", Message: "call names must be unique within a workflow"}
	resp := doJSON(t, m, true, http.MethodPost, callsPath(), validCallBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["code"] != "INVALID_WORKFLOW" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestCallHandler_Create_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.create.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, callsPath(), validCallBody())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to create call" {
		t.Fatalf("unexpected body %v", out)
	}
}
