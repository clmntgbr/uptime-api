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
	if cmd.UserID != testutil.TestUserID || def.Name != "Checkout" || def.ScheduleType != domainworkflow.ScheduleInterval || def.Interval != 5*time.Minute || !def.Enabled {
		t.Fatalf("unexpected command %+v", cmd)
	}
	if len(def.Statics) != 0 || len(def.Calls) != 0 {
		t.Fatalf("create must not accept statics or calls in the body, got statics=%+v calls=%+v", def.Statics, def.Calls)
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

func TestWorkflowHandler_Create_DefaultScheduleTypeInterval(t *testing.T) {
	m := newMocks()
	body := map[string]any{"name": "Checkout", "intervalSeconds": 300}
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
	assertStatus(t, resp, http.StatusCreated)
	if m.create.got.Definition.ScheduleType != domainworkflow.ScheduleInterval {
		t.Fatalf("missing scheduleType must default to interval, got %q", m.create.got.Definition.ScheduleType)
	}
}

func TestWorkflowHandler_Create_Once_Success(t *testing.T) {
	m := newMocks()
	body := map[string]any{"name": "One shot", "scheduleType": "once"}
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
	assertStatus(t, resp, http.StatusCreated)
	if m.create.got.Definition.ScheduleType != domainworkflow.ScheduleOnce || m.create.got.Definition.Interval != 0 {
		t.Fatalf("unexpected once definition %+v", m.create.got.Definition)
	}
}

func TestWorkflowHandler_Create_At_Success(t *testing.T) {
	m := newMocks()
	runAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Minute)
	body := map[string]any{
		"name":         "Timed",
		"scheduleType": "at",
		"runAt":        runAt.Format(time.RFC3339),
	}
	resp := doJSON(t, m, true, http.MethodPost, "/workflows", body)
	assertStatus(t, resp, http.StatusCreated)
	def := m.create.got.Definition
	if def.ScheduleType != domainworkflow.ScheduleAt || def.RunAt == nil || !def.RunAt.Equal(runAt) {
		t.Fatalf("unexpected at definition %+v", def)
	}
}

func TestWorkflowHandler_Create_ValidationFailed(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Minute).Format(time.RFC3339)
	cases := map[string]func(map[string]any){
		"interval too low":      func(b map[string]any) { b["intervalSeconds"] = 30 },
		"missing name":          func(b map[string]any) { b["name"] = "" },
		"once with interval":    func(b map[string]any) { b["scheduleType"] = "once" },
		"once with runAt":       func(b map[string]any) { b["scheduleType"] = "once"; delete(b, "intervalSeconds"); b["runAt"] = future },
		"at without runAt":      func(b map[string]any) { b["scheduleType"] = "at"; delete(b, "intervalSeconds") },
		"at with interval":      func(b map[string]any) { b["scheduleType"] = "at"; b["runAt"] = future },
		"interval with runAt":   func(b map[string]any) { b["runAt"] = future },
		"interval without secs": func(b map[string]any) { delete(b, "intervalSeconds") },
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
