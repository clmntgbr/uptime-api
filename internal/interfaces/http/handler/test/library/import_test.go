package librarytest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	querylibrary "go-api/internal/application/query/library"
	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func previewBody(format, content string) map[string]any {
	return map[string]any{"format": format, "content": content}
}

func TestLibraryHandler_PreviewImport_Success(t *testing.T) {
	m := newMocks()
	m.preview.result = &querylibrary.PreviewImportResult{
		Candidates: []domainlibrary.Candidate{{
			Key:        "op-1",
			Definition: validDefinition(),
			Warnings:   []domainlibrary.Warning{{Code: "sensitive_header", Field: "headers.Authorization", Message: "literal token"}},
		}},
		Skipped: []domainlibrary.Skipped{{Source: "POST /legacy", Reason: "multipart not supported"}},
	}

	resp := doJSON(t, m, true, http.MethodPost, previewPath, previewBody("curl", "curl https://example.com"))
	assertStatus(t, resp, http.StatusOK)

	var out presenter.LibraryImportPreviewResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Items) != 1 || out.Items[0].Key != "op-1" || out.Items[0].Payload.Method != "GET" {
		t.Fatalf("unexpected items %+v", out.Items)
	}
	if len(out.Items[0].Warnings) != 1 || out.Items[0].Warnings[0].Code != "sensitive_header" {
		t.Fatalf("unexpected warnings %+v", out.Items[0].Warnings)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].Source != "POST /legacy" {
		t.Fatalf("unexpected skipped %+v", out.Skipped)
	}
	if m.preview.got.Format != domainlibrary.FormatCurl || string(m.preview.got.Content) != "curl https://example.com" {
		t.Fatalf("unexpected query %+v", m.preview.got)
	}
}

func TestLibraryHandler_PreviewImport_EmptyLists(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, previewPath, previewBody("postman", `{}`))
	assertStatus(t, resp, http.StatusOK)
	out := testutil.DecodeJSONMap(t, resp)
	if items, ok := out["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items must be [], got %v", out["items"])
	}
	if skipped, ok := out["skipped"].([]any); !ok || len(skipped) != 0 {
		t.Fatalf("skipped must be [], got %v", out["skipped"])
	}
}

func TestLibraryHandler_PreviewImport_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, previewPath, `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.preview.calls != 0 {
		t.Fatal("preview must not run on invalid body")
	}
}

func TestLibraryHandler_PreviewImport_ValidationFailed(t *testing.T) {
	cases := map[string]map[string]any{
		"missing content": {"format": "curl"},
		"unknown format":  {"format": "har", "content": "x"},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			resp := doJSON(t, m, true, http.MethodPost, previewPath, body)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.preview.calls != 0 {
				t.Fatal("preview must not run on invalid payload")
			}
		})
	}
}

func TestLibraryHandler_PreviewImport_HandlerErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"too large", domainlibrary.ErrImportTooLarge, http.StatusRequestEntityTooLarge},
		{"unsupported format", domainlibrary.ErrUnsupportedFormat, http.StatusBadRequest},
		{"import error", &domainlibrary.ImportError{Message: "could not parse OpenAPI document"}, http.StatusUnprocessableEntity},
		{"internal", errDatabase, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMocks()
			m.preview.err = tc.err
			resp := doJSON(t, m, true, http.MethodPost, previewPath, previewBody("openapi", "{}"))
			assertStatus(t, resp, tc.status)
			if out := testutil.DecodeJSONMap(t, resp); out["message"] == nil {
				t.Fatalf("message missing: %v", out)
			}
		})
	}
}

func confirmItem(key, name string, rules []map[string]any) map[string]any {
	payload := validBody()
	payload["name"] = "Source name"
	if rules != nil {
		payload["rules"] = rules
	}
	return map[string]any{"key": key, "name": name, "payload": payload}
}

func TestLibraryHandler_Import_Success(t *testing.T) {
	m := newMocks()
	body := map[string]any{"items": []map[string]any{
		confirmItem("op-1", "Renamed", nil),
		confirmItem("op-2", "", nil),
	}}

	resp := doJSON(t, m, true, http.MethodPost, importPath, body)
	assertStatus(t, resp, http.StatusCreated)

	var out []presenter.LibraryItemResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 2 {
		t.Fatalf("expected 2 created items, got %d", len(out))
	}
	cmd := m.imports.got
	if cmd.UserID != testutil.TestUserID || len(cmd.Items) != 2 {
		t.Fatalf("unexpected command %+v", cmd)
	}
	if cmd.Items[0].Key != "op-1" || cmd.Items[0].Definition.Name != "Renamed" {
		t.Fatalf("name override not applied: %+v", cmd.Items[0])
	}
	if cmd.Items[1].Definition.Name != "Source name" {
		t.Fatalf("payload name must be kept without override: %+v", cmd.Items[1])
	}
}

func TestLibraryHandler_Import_Unauthorized(t *testing.T) {
	m := newMocks()
	body := map[string]any{"items": []map[string]any{confirmItem("op-1", "", nil)}}
	resp := doJSON(t, m, false, http.MethodPost, importPath, body)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_Import_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, importPath, `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.imports.calls != 0 {
		t.Fatal("import must not run on invalid body")
	}
}

func TestLibraryHandler_Import_ValidationFailed(t *testing.T) {
	cases := map[string]map[string]any{
		"no items": {"items": []map[string]any{}},
		"too many items": {"items": func() []map[string]any {
			items := make([]map[string]any, 0, 51)
			for i := 0; i < 51; i++ {
				items = append(items, confirmItem(fmt.Sprintf("op-%d", i), "", nil))
			}
			return items
		}()},
		"name too long": {"items": []map[string]any{confirmItem("op-1", string(make([]byte, 101)), nil)}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMocks()
			resp := doJSON(t, m, true, http.MethodPost, importPath, body)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.imports.calls != 0 {
				t.Fatal("import must not run on invalid payload")
			}
		})
	}
}

func TestLibraryHandler_Import_RuleErrorsAreKeyedByItem(t *testing.T) {
	m := newMocks()
	bad := []map[string]any{{"subject": "status", "operator": "exceeds", "value": "200"}}
	body := map[string]any{"items": []map[string]any{
		confirmItem("op-1", "", nil),
		confirmItem("op-2", "", bad),
	}}
	resp := doJSON(t, m, true, http.MethodPost, importPath, body)
	assertStatus(t, resp, http.StatusBadRequest)
	errs, _ := testutil.DecodeJSONMap(t, resp)["errors"].(map[string]any)
	if _, ok := errs["items[1].payload.rules[0]"]; !ok {
		t.Fatalf("expected items[1].payload.rules[0] error, got %v", errs)
	}
	if m.imports.calls != 0 {
		t.Fatal("import must not run when a rule is invalid")
	}
}

func TestLibraryHandler_Import_HandlerError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"invalid call", &domaincall.ValidationError{Call: "Health", Message: "url is required"}, http.StatusBadRequest},
		{"internal", errDatabase, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMocks()
			m.imports.err = tc.err
			body := map[string]any{"items": []map[string]any{confirmItem("op-1", "", nil)}}
			resp := doJSON(t, m, true, http.MethodPost, importPath, body)
			assertStatus(t, resp, tc.status)
		})
	}
}

func TestLibraryHandler_PreviewImport_PassesBaseURL(t *testing.T) {
	m := newMocks()
	body := previewBody("openapi", "{}")
	body["baseUrl"] = "https://api.example.com"
	resp := doJSON(t, m, true, http.MethodPost, previewPath, body)
	assertStatus(t, resp, http.StatusOK)
	if m.preview.got.BaseURL != "https://api.example.com" {
		t.Fatalf("base url not passed: %+v", m.preview.got)
	}
}

func TestLibraryHandler_PreviewImport_InvalidBaseURL(t *testing.T) {
	m := newMocks()
	m.preview.err = domainlibrary.ErrInvalidBaseURL
	body := previewBody("openapi", "{}")
	body["baseUrl"] = "api.example.com"
	resp := doJSON(t, m, true, http.MethodPost, previewPath, body)
	assertStatus(t, resp, http.StatusBadRequest)

	m = newMocks()
	body["baseUrl"] = strings.Repeat("a", 2049)
	resp = doJSON(t, m, true, http.MethodPost, previewPath, body)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.preview.calls != 0 {
		t.Fatal("preview must not run with too long base url")
	}
}
