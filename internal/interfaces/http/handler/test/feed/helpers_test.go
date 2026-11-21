package feedtest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	queryfeed "go-api/internal/application/query/feed"
	domainfeed "go-api/internal/domain/feed"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
)

var errDatabase = errors.New("pq: connection refused")

type mockList struct {
	calls int
	got   queryfeed.ListFeedQuery
	out   queryfeed.ListFeedResult
	err   error
}

func (m *mockList) Handle(_ context.Context, q queryfeed.ListFeedQuery) (queryfeed.ListFeedResult, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

func mount(list *mockList, auth bool) *fiber.App {
	h := handler.NewFeedHandler(list)
	app := testutil.NewTestApp()
	if auth {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/feed", h.List)
	return app
}

func doJSON(t *testing.T, list *mockList, auth bool, path string) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	app := mount(list, auth)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status=%d want=%d", resp.StatusCode, want)
	}
}

func sampleEvent() domainfeed.Event {
	wf := testutil.TestProjectID
	return domainfeed.Event{
		ID: testutil.TestUserID, UserID: testutil.TestUserID, WorkflowID: &wf,
		Kind: domainfeed.KindCallFailed, Severity: domainfeed.SeverityCritical,
		Title: "Checkout failed (502)", Payload: map[string]any{"httpStatus": 502},
	}
}
