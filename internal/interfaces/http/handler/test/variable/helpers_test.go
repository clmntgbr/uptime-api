package variabletest

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	cmdvariable "go-api/internal/application/command/variable"
	queryvariable "go-api/internal/application/query/variable"
	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000b1")
	testCallID     = uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	testVariableID = uuid.MustParse("01960000-0000-7000-8000-0000000000d1")
	errDatabase    = errors.New("pq: connection refused")
)

type mockCreateGlobal struct {
	calls int
	got   cmdvariable.CreateGlobalCommand
	out   *variable.Global
	err   error
}

func (m *mockCreateGlobal) Handle(_ context.Context, cmd cmdvariable.CreateGlobalCommand) (*variable.Global, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &variable.Global{ID: testVariableID, Name: cmd.Name, Value: cmd.Value, Secret: cmd.Secret}, nil
}

type mockUpdateGlobal struct {
	calls int
	got   cmdvariable.UpdateGlobalCommand
	out   *variable.Global
	err   error
}

func (m *mockUpdateGlobal) Handle(_ context.Context, cmd cmdvariable.UpdateGlobalCommand) (*variable.Global, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &variable.Global{ID: cmd.ID, Name: "token", Value: cmd.Value, Secret: cmd.Secret}, nil
}

type mockDeleteGlobal struct {
	calls int
	got   cmdvariable.DeleteGlobalVariableCommand
	err   error
}

func (m *mockDeleteGlobal) Handle(_ context.Context, cmd cmdvariable.DeleteGlobalVariableCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockListGlobals struct {
	calls int
	out   []variable.Global
	err   error
}

func (m *mockListGlobals) Handle(_ context.Context, _ queryvariable.ListGlobalsQuery) ([]variable.Global, error) {
	m.calls++
	return m.out, m.err
}

type mockGetGlobal struct {
	calls int
	got   queryvariable.GetGlobalQuery
	out   *variable.Global
	err   error
}

func (m *mockGetGlobal) Handle(_ context.Context, q queryvariable.GetGlobalQuery) (*variable.Global, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mockCreateStatic struct {
	calls int
	got   cmdvariable.CreateStaticCommand
	out   *domainworkflow.StaticVariable
	err   error
}

func (m *mockCreateStatic) Handle(_ context.Context, cmd cmdvariable.CreateStaticCommand) (*domainworkflow.StaticVariable, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &domainworkflow.StaticVariable{Name: cmd.Name, Value: cmd.Value}, nil
}

type mockUpdateStatic struct {
	calls int
	got   cmdvariable.UpdateStaticCommand
	out   *domainworkflow.StaticVariable
	err   error
}

func (m *mockUpdateStatic) Handle(_ context.Context, cmd cmdvariable.UpdateStaticCommand) (*domainworkflow.StaticVariable, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &domainworkflow.StaticVariable{Name: cmd.Name, Value: cmd.Value}, nil
}

type mockDeleteStatic struct {
	calls int
	got   cmdvariable.DeleteStaticCommand
	err   error
}

func (m *mockDeleteStatic) Handle(_ context.Context, cmd cmdvariable.DeleteStaticCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockListStatics struct {
	calls int
	out   []domainworkflow.StaticVariable
	err   error
}

func (m *mockListStatics) Handle(_ context.Context, _ queryvariable.ListStaticsQuery) ([]domainworkflow.StaticVariable, error) {
	m.calls++
	return m.out, m.err
}

type mockGetStatic struct {
	calls int
	got   queryvariable.GetStaticQuery
	out   *domainworkflow.StaticVariable
	err   error
}

func (m *mockGetStatic) Handle(_ context.Context, q queryvariable.GetStaticQuery) (*domainworkflow.StaticVariable, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mockCreateCall struct {
	calls int
	got   cmdvariable.CreateCallVariableCommand
	out   *domaincall.Extraction
	err   error
}

func (m *mockCreateCall) Handle(_ context.Context, cmd cmdvariable.CreateCallVariableCommand) (*domaincall.Extraction, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &domaincall.Extraction{Name: cmd.Name, Source: domaincall.ExtractJSON, Expression: cmd.Path}, nil
}

type mockUpdateCall struct {
	calls int
	got   cmdvariable.UpdateCallVariableCommand
	out   *domaincall.Extraction
	err   error
}

func (m *mockUpdateCall) Handle(_ context.Context, cmd cmdvariable.UpdateCallVariableCommand) (*domaincall.Extraction, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	if m.out != nil {
		return m.out, nil
	}
	return &domaincall.Extraction{Name: cmd.Name, Source: domaincall.ExtractJSON, Expression: cmd.Path}, nil
}

type mockDeleteCall struct {
	calls int
	got   cmdvariable.DeleteCallVariableCommand
	err   error
}

func (m *mockDeleteCall) Handle(_ context.Context, cmd cmdvariable.DeleteCallVariableCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockListCall struct {
	calls int
	out   []domaincall.Extraction
	err   error
}

func (m *mockListCall) Handle(_ context.Context, _ queryvariable.ListCallVariablesQuery) ([]domaincall.Extraction, error) {
	m.calls++
	return m.out, m.err
}

type mockGetCall struct {
	calls int
	got   queryvariable.GetCallVariableQuery
	out   *domaincall.Extraction
	err   error
}

func (m *mockGetCall) Handle(_ context.Context, q queryvariable.GetCallVariableQuery) (*domaincall.Extraction, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mockListAvailable struct {
	calls int
	got   queryvariable.ListAvailableQuery
	out   []variable.Available
	err   error
}

func (m *mockListAvailable) Handle(_ context.Context, q queryvariable.ListAvailableQuery) ([]variable.Available, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mockListResponseKeys struct {
	calls int
	got   queryvariable.ListResponseKeysQuery
	out   []string
	err   error
}

func (m *mockListResponseKeys) Handle(_ context.Context, q queryvariable.ListResponseKeysQuery) ([]string, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.out, nil
}

type mocks struct {
	createGlobal  *mockCreateGlobal
	updateGlobal  *mockUpdateGlobal
	deleteGlobal  *mockDeleteGlobal
	listGlobals   *mockListGlobals
	getGlobal     *mockGetGlobal
	createStatic  *mockCreateStatic
	updateStatic  *mockUpdateStatic
	deleteStatic  *mockDeleteStatic
	listStatics   *mockListStatics
	getStatic     *mockGetStatic
	createCall    *mockCreateCall
	updateCall    *mockUpdateCall
	deleteCall    *mockDeleteCall
	listCall         *mockListCall
	getCall          *mockGetCall
	listAvailable    *mockListAvailable
	listResponseKeys *mockListResponseKeys
}

func newMocks() mocks {
	return mocks{
		createGlobal: &mockCreateGlobal{}, updateGlobal: &mockUpdateGlobal{}, deleteGlobal: &mockDeleteGlobal{},
		listGlobals: &mockListGlobals{}, getGlobal: &mockGetGlobal{},
		createStatic: &mockCreateStatic{}, updateStatic: &mockUpdateStatic{}, deleteStatic: &mockDeleteStatic{},
		listStatics: &mockListStatics{}, getStatic: &mockGetStatic{},
		createCall: &mockCreateCall{}, updateCall: &mockUpdateCall{}, deleteCall: &mockDeleteCall{},
		listCall: &mockListCall{}, getCall: &mockGetCall{}, listAvailable: &mockListAvailable{},
		listResponseKeys: &mockListResponseKeys{},
	}
}

func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewVariableHandler(
		m.createGlobal, m.updateGlobal, m.deleteGlobal, m.listGlobals, m.getGlobal,
		m.createStatic, m.updateStatic, m.deleteStatic, m.listStatics, m.getStatic,
		m.createCall, m.updateCall, m.deleteCall, m.listCall, m.getCall, m.listAvailable,
		m.listResponseKeys,
	)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/variables", h.ListGlobals)
	app.Post("/variables", h.CreateGlobal)
	app.Get("/variables/:id", h.GetGlobal)
	app.Put("/variables/:id", h.UpdateGlobal)
	app.Delete("/variables/:id", h.DeleteGlobal)

	app.Get("/workflows/:id/variables", h.ListStatics)
	app.Post("/workflows/:id/variables", h.CreateStatic)
	app.Get("/workflows/:id/variables/:name", h.GetStatic)
	app.Put("/workflows/:id/variables/:name", h.UpdateStatic)
	app.Delete("/workflows/:id/variables/:name", h.DeleteStatic)

	app.Get("/workflows/:id/calls/:callId/available-variables", h.ListAvailable)
	app.Get("/workflows/:id/calls/:callId/response-keys", h.ListResponseKeys)
	app.Get("/workflows/:id/calls/:callId/variables", h.ListCallVariables)
	app.Post("/workflows/:id/calls/:callId/variables", h.CreateCallVariable)
	app.Get("/workflows/:id/calls/:callId/variables/:name", h.GetCallVariable)
	app.Put("/workflows/:id/calls/:callId/variables/:name", h.UpdateCallVariable)
	app.Delete("/workflows/:id/calls/:callId/variables/:name", h.DeleteCallVariable)
	return app
}

func globalVarPath() string {
	return "/variables/" + testVariableID.String()
}

func workflowVarsPath() string {
	return "/workflows/" + testWorkflowID.String() + "/variables"
}

func callVarsPath() string {
	return "/workflows/" + testWorkflowID.String() + "/calls/" + testCallID.String() + "/variables"
}

func availablePath() string {
	return "/workflows/" + testWorkflowID.String() + "/calls/" + testCallID.String() + "/available-variables"
}

func responseKeysPath() string {
	return "/workflows/" + testWorkflowID.String() + "/calls/" + testCallID.String() + "/response-keys"
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
