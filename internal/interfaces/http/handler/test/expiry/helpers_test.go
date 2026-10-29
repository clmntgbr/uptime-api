package expirytest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cmdexpiry "go-api/internal/application/command/expiry"
	queryexpiry "go-api/internal/application/query/expiry"
	domainexpiry "go-api/internal/domain/expiry"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000f1")
	errDatabase    = errors.New("pq: connection refused")
)

type mockListPolicies struct {
	calls int
	got   queryexpiry.ListPoliciesQuery
	out   []domainexpiry.Policy
	err   error
}

func (m *mockListPolicies) Handle(_ context.Context, q queryexpiry.ListPoliciesQuery) ([]domainexpiry.Policy, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mockUpsertPolicy struct {
	calls int
	got   cmdexpiry.UpsertPolicyCommand
	out   domainexpiry.Policy
	err   error
}

func (m *mockUpsertPolicy) Handle(_ context.Context, cmd cmdexpiry.UpsertPolicyCommand) (domainexpiry.Policy, error) {
	m.calls++
	m.got = cmd
	return m.out, m.err
}

type mockListItems struct {
	calls int
	got   queryexpiry.ListItemsQuery
	out   []domainexpiry.ItemView
	err   error
}

func (m *mockListItems) Handle(_ context.Context, q queryexpiry.ListItemsQuery) ([]domainexpiry.ItemView, error) {
	m.calls++
	m.got = q
	return m.out, m.err
}

type mocks struct {
	listPolicies *mockListPolicies
	upsert       *mockUpsertPolicy
	listItems    *mockListItems
}

func newMocks() mocks {
	return mocks{
		listPolicies: &mockListPolicies{},
		upsert:       &mockUpsertPolicy{},
		listItems:    &mockListItems{},
	}
}

func mount(m mocks, auth bool) *fiber.App {
	h := handler.NewExpiryHandler(m.listPolicies, m.upsert, m.listItems)
	app := testutil.NewTestApp()
	if auth {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows/:id/expiry-policies", h.ListPolicies)
	app.Put("/workflows/:id/expiry-policies/:type", h.UpsertPolicy)
	app.Get("/workflows/:id/expiry-items", h.ListItems)
	return app
}

func doJSON(t *testing.T, m mocks, auth bool, method, path string, body any) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(method, path, body)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := mount(m, auth).Test(req)
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status %d want %d", resp.StatusCode, want)
	}
}
