package librarytest

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	cmdlibrary "go-api/internal/application/command/library"
	querylibrary "go-api/internal/application/query/library"
	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testItemID  = uuid.MustParse("01960000-0000-7000-8000-0000000000d1")
	testCallID  = uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	testTime    = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	errDatabase = errors.New("pq: connection refused")
	libraryPath = "/library"
	libraryItem = libraryPath + "/" + testItemID.String()
	importPath  = libraryPath + "/import"
	previewPath = libraryPath + "/import/preview"
)

func sampleItem(def domaincall.Definition) *domainlibrary.Item {
	return &domainlibrary.Item{
		ID:         testItemID,
		UserID:     testutil.TestUserID,
		Definition: def,
		CreatedAt:  testTime,
		UpdatedAt:  testTime,
	}
}

type mockCreate struct {
	calls int
	got   cmdlibrary.CreateItemCommand
	err   error
}

func (m *mockCreate) Handle(_ context.Context, cmd cmdlibrary.CreateItemCommand) (*domainlibrary.Item, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return sampleItem(cmd.Definition), nil
}

type mockUpdate struct {
	calls int
	got   cmdlibrary.UpdateItemCommand
	err   error
}

func (m *mockUpdate) Handle(_ context.Context, cmd cmdlibrary.UpdateItemCommand) (*domainlibrary.Item, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return sampleItem(cmd.Definition), nil
}

type mockDelete struct {
	calls int
	got   cmdlibrary.DeleteItemCommand
	err   error
}

func (m *mockDelete) Handle(_ context.Context, cmd cmdlibrary.DeleteItemCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockImport struct {
	calls int
	got   cmdlibrary.ImportCommand
	err   error
}

func (m *mockImport) Handle(_ context.Context, cmd cmdlibrary.ImportCommand) ([]domainlibrary.Item, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	out := make([]domainlibrary.Item, 0, len(cmd.Items))
	for _, in := range cmd.Items {
		out = append(out, *sampleItem(in.Definition))
	}
	return out, nil
}

type mockList struct {
	calls int
	got   querylibrary.ListItemsQuery
	views []domainlibrary.ListView
	total int64
	err   error
}

func (m *mockList) Handle(_ context.Context, q querylibrary.ListItemsQuery) ([]domainlibrary.ListView, int64, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.views, m.total, nil
}

type mockGet struct {
	calls int
	got   querylibrary.GetItemQuery
	item  *domainlibrary.Item
	err   error
}

func (m *mockGet) Handle(_ context.Context, q querylibrary.GetItemQuery) (*domainlibrary.Item, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.item, nil
}

type mockPreview struct {
	calls  int
	got    querylibrary.PreviewImportQuery
	result *querylibrary.PreviewImportResult
	err    error
}

func (m *mockPreview) Handle(_ context.Context, q querylibrary.PreviewImportQuery) (*querylibrary.PreviewImportResult, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &querylibrary.PreviewImportResult{}, nil
}

type mockSaveFromCall struct {
	calls int
	got   cmdlibrary.SaveFromCallCommand
	err   error
}

func (m *mockSaveFromCall) Handle(_ context.Context, cmd cmdlibrary.SaveFromCallCommand) (*domainlibrary.Item, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return sampleItem(validDefinition()), nil
}

type mocks struct {
	saveFromCall *mockSaveFromCall
	create       *mockCreate
	update       *mockUpdate
	delete       *mockDelete
	imports      *mockImport
	list         *mockList
	get          *mockGet
	preview      *mockPreview
}

func newMocks() mocks {
	return mocks{
		create:       &mockCreate{},
		update:       &mockUpdate{},
		delete:       &mockDelete{},
		imports:      &mockImport{},
		list:         &mockList{},
		get:          &mockGet{item: sampleItem(validDefinition())},
		preview:      &mockPreview{},
		saveFromCall: &mockSaveFromCall{},
	}
}

func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewLibraryHandler(m.create, m.update, m.delete, m.imports, m.list, m.get, m.preview, m.saveFromCall)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/library", h.List)
	app.Post("/library", h.Create)
	app.Post("/library/import/preview", h.PreviewImport)
	app.Post("/library/import", h.Import)
	app.Get("/library/:id", h.GetByID)
	app.Put("/library/:id", h.Update)
	app.Delete("/library/:id", h.Delete)
	app.Post("/library/from-call/:callId", h.SaveFromCall)
	return app
}

func validDefinition() domaincall.Definition {
	return domaincall.Definition{
		Name:      "Health",
		Method:    "GET",
		URL:       "https://api.example.com/health",
		Timeout:   5 * time.Second,
		IPVersion: domaincall.IPAny,
	}
}

func validBody() map[string]any {
	return map[string]any{
		"name":      "Health",
		"method":    "GET",
		"url":       "https://api.example.com/health",
		"timeoutMs": 5000,
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
