package workflowtest

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	domainrule "go-api/internal/domain/rule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func doJSON(t *testing.T, m mocks, authenticated bool, method, path string, body any) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(method, path, body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := mountRoutes(m, authenticated).Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	return resp
}

func doRaw(t *testing.T, m mocks, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := mountRoutes(m, true).Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status: got %d want %d", resp.StatusCode, want)
	}
}

func TestWorkflowHandler_Create_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", validWorkflowBody())
	assertStatus(t, resp, http.StatusCreated)

	var out presenter.WorkflowIDResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testWorkflowID.String() {
		t.Fatalf("response id: got %q", out.ID)
	}

	if m.create.calls != 1 {
		t.Fatalf("create calls: got %d want 1", m.create.calls)
	}
	cmd := m.create.got
	def := cmd.Definition
	if cmd.UserID != testutil.TestUserID || def.Name != "Checkout" || def.Interval != 5*time.Minute || !def.Enabled {
		t.Fatalf("unexpected command %+v", cmd)
	}
	if len(def.Statics) != 1 || def.Statics[0].Name != "base_url" {
		t.Fatalf("unexpected statics %+v", def.Statics)
	}
	login := def.Calls[0]
	if len(def.Calls) != 2 || login.Method != "POST" || login.Timeout != 5*time.Second ||
		login.VerifyTLS == nil || *login.VerifyTLS || login.IPVersion != domaincall.IPv4 {
		t.Fatalf("unexpected login call %+v", login)
	}
	if len(login.Extractions) != 1 || login.Extractions[0].Source != domaincall.ExtractJSON || login.Extractions[0].Expression != "$.access_token" {
		t.Fatalf("unexpected extractions %+v", login.Extractions)
	}
	if len(login.Rules) != 2 || login.Rules[1].Subject != domainrule.SubjectResponseTime || login.Rules[1].Value != "800" {
		t.Fatalf("rules must be built and normalized: %+v", login.Rules)
	}
	if orders := def.Calls[1]; orders.VerifyTLS != nil || len(orders.Rules) != 0 || orders.Timeout != 0 {
		t.Fatalf("omitted fields must stay unset for domain defaults: %+v", orders)
	}
}

func TestWorkflowHandler_Create_DisabledWorkflow(t *testing.T) {
	m := newMocks()
	body := validWorkflowBody()
	body["enabled"] = false
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
	assertStatus(t, resp, http.StatusCreated)
	if m.create.got.Definition.Enabled {
		t.Fatal("enabled=false must be passed to the command")
	}
}

func TestWorkflowHandler_Create_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, "/workflows", validWorkflowBody())
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.create.calls != 0 {
		t.Fatal("create must not be called without a user")
	}
}

func TestWorkflowHandler_Create_InvalidJSON(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, "/workflows", `{"name":`)
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid request body" {
		t.Fatalf("unexpected body %v", out)
	}
	if m.create.calls != 0 {
		t.Fatal("create must not be called on invalid JSON")
	}
}

func TestWorkflowHandler_Create_ValidationFailed(t *testing.T) {
	cases := map[string]func(map[string]any){
		"missing calls":    func(b map[string]any) { delete(b, "calls") },
		"interval too low": func(b map[string]any) { b["intervalSeconds"] = 30 },
		"missing name":     func(b map[string]any) { b["name"] = "" },
		"bad method":       func(b map[string]any) { b["calls"].([]map[string]any)[0]["method"] = "TRACE" },
		"bad rule operator": func(b map[string]any) {
			b["calls"].([]map[string]any)[0]["rules"] = []map[string]any{{"subject": "status", "operator": "huge"}}
		},
		"bad extraction": func(b map[string]any) {
			b["calls"].([]map[string]any)[0]["extractions"] = []map[string]any{{"name": "x", "source": "xml"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			body := validWorkflowBody()
			mutate(body)
			resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
			assertStatus(t, resp, http.StatusBadRequest)
			out := testutil.DecodeJSONMap(t, resp)
			if out["message"] != "Validation failed" || out["errors"] == nil {
				t.Fatalf("expected field errors, got %v", out)
			}
			if m.create.calls != 0 {
				t.Fatal("create must not be called on invalid payload")
			}
		})
	}
}

func TestWorkflowHandler_Create_InvalidRuleValue(t *testing.T) {
	m := newMocks()
	body := validWorkflowBody()
	body["calls"].([]map[string]any)[1]["rules"] = []map[string]any{
		{"subject": "status", "operator": "is", "value": "200"},
		{"subject": "status", "operator": "exceeds", "value": "200"},
	}
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
	assertStatus(t, resp, http.StatusBadRequest)

	out := testutil.DecodeJSONMap(t, resp)
	errs, _ := out["errors"].(map[string]any)
	if _, ok := errs["calls[1].rules[1]"]; !ok || len(errs) != 1 {
		t.Fatalf("expected an error on calls[1].rules[1], got %v", out)
	}
	if m.create.calls != 0 {
		t.Fatal("create must not be called with an invalid rule")
	}
}

func TestWorkflowHandler_Create_HandlerError_InvalidWorkflow(t *testing.T) {
	m := newMocks()
	m.create.err = fmt.Errorf("create workflow: %w", &domainworkflow.ValidationError{
		Call: "orders", Variable: "token", Message: "unknown variable", Kind: domainworkflow.ErrUnknownVar,
	})
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", validWorkflowBody())
	assertStatus(t, resp, http.StatusBadRequest)

	out := testutil.DecodeJSONMap(t, resp)
	message, _ := out["message"].(string)
	if out["code"] != "INVALID_WORKFLOW" || !strings.Contains(message, `call "orders"`) || strings.Contains(message, "create workflow") {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestWorkflowHandler_Create_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.create.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", validWorkflowBody())
	assertStatus(t, resp, http.StatusInternalServerError)

	out := testutil.DecodeJSONMap(t, resp)
	if out["message"] != "Failed to create workflow" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
