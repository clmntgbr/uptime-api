package workflowtest

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

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
	if len(def.Calls) != 0 {
		t.Fatalf("create must not accept calls in the body, got %+v", def.Calls)
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
		"interval too low": func(b map[string]any) { b["intervalSeconds"] = 30 },
		"missing name":     func(b map[string]any) { b["name"] = "" },
		"static without name": func(b map[string]any) {
			b["statics"] = []map[string]any{{"name": "", "value": "x"}}
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

func TestWorkflowHandler_Create_HandlerError_InvalidWorkflow(t *testing.T) {
	m := newMocks()
	m.create.err = fmt.Errorf("create workflow: %w", &domainworkflow.ValidationError{
		Message: "interval must be a whole number of minutes, at least 1 minute",
	})
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", validWorkflowBody())
	assertStatus(t, resp, http.StatusBadRequest)

	out := testutil.DecodeJSONMap(t, resp)
	message, _ := out["message"].(string)
	if out["code"] != "INVALID_WORKFLOW" || !strings.Contains(message, "interval") || strings.Contains(message, "create workflow") {
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
