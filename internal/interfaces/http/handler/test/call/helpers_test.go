package calltest

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	callcmd "go-api/internal/application/command/call"
	querycall "go-api/internal/application/query/call"
	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000b1")
	testCallID     = uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	errDatabase    = errors.New("pq: connection refused")
)

type mockCreateHandler struct {
	calls int
	got   callcmd.CreateCallCommand
	err   error
}

func (m *mockCreateHandler) Handle(_ context.Context, cmd callcmd.CreateCallCommand) (*domaincall.Call, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return &domaincall.Call{ID: testCallID, Name: cmd.Definition.Name}, nil
}

type mockUpdateHandler struct {
	calls int
	got   callcmd.UpdateCallCommand
	err   error
}

func (m *mockUpdateHandler) Handle(_ context.Context, cmd callcmd.UpdateCallCommand) (*domaincall.Call, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return &domaincall.Call{ID: cmd.CallID, Name: cmd.Definition.Name}, nil
}

type mockDeleteHandler struct {
	calls int
	got   callcmd.DeleteCallCommand
	err   error
}

func (m *mockDeleteHandler) Handle(_ context.Context, cmd callcmd.DeleteCallCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockListHandler struct {
	calls int
	got   querycall.ListCallsQuery
	views []domainworkflow.CallListView
	err   error
}

func (m *mockListHandler) Handle(_ context.Context, q querycall.ListCallsQuery) ([]domainworkflow.CallListView, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.views, nil
}

type mockGetHandler struct {
	calls int
	got   querycall.GetCallQuery
	view  *domainworkflow.CallView
	err   error
}

func (m *mockGetHandler) Handle(_ context.Context, q querycall.GetCallQuery) (*domainworkflow.CallView, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.view, nil
}

func sampleCallView() *domainworkflow.CallView {
	return &domainworkflow.CallView{
		ID:        testCallID,
		Name:      "Google health",
		Method:    "GET",
		URL:       "https://httpbin.org/json",
		Headers:   map[string]string{},
		Query:     map[string]string{},
		Body:      "",
		TimeoutMS: 30000,
		VerifyTLS: true,
		IPVersion: "any",
		ProxyURL:  "",
	}
}

type mocks struct {
	create *mockCreateHandler
	update *mockUpdateHandler
	delete *mockDeleteHandler
	list   *mockListHandler
	get    *mockGetHandler
}

func newMocks() mocks {
	return mocks{
		create: &mockCreateHandler{},
		update: &mockUpdateHandler{},
		delete: &mockDeleteHandler{},
		list:   &mockListHandler{},
		get:    &mockGetHandler{},
	}
}

func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewCallHandler(m.create, m.update, m.delete, m.list, m.get)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows/:id/calls", h.List)
	app.Post("/workflows/:id/calls", h.Create)
	app.Get("/workflows/:id/calls/:callId", h.GetByID)
	app.Put("/workflows/:id/calls/:callId", h.Update)
	app.Delete("/workflows/:id/calls/:callId", h.Delete)
	return app
}

func callsPath() string {
	return "/workflows/" + testWorkflowID.String() + "/calls"
}

func callPath() string {
	return callsPath() + "/" + testCallID.String()
}

func validCallBody() map[string]any {
	return map[string]any{
		"name":      "login",
		"method":    "POST",
		"url":       "https://api.example.com/login",
		"headers":   map[string]string{"Content-Type": "application/json"},
		"body":      `{"user":"demo"}`,
		"timeoutMs": 5000,
		"verifyTls": true,
		"ipVersion": "any",
		"rules": []map[string]any{
			{"subject": "status", "operator": "is", "value": "200"},
		},
	}
}

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
