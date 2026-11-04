package incidenttest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cmdincident "go-api/internal/application/command/incident"
	queryincident "go-api/internal/application/query/incident"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	testIncidentID = uuid.MustParse("01960000-0000-7000-8000-0000000000c2")
	errDatabase    = errors.New("pq: connection refused")
)

type mockListPolicies struct {
	calls int
	out   []domainincident.Policy
	err   error
}

func (m *mockListPolicies) Handle(_ context.Context, _ queryincident.ListPoliciesQuery) ([]domainincident.Policy, error) {
	m.calls++
	return m.out, m.err
}

type mockUpsertPolicy struct {
	calls int
	got   cmdincident.UpsertPolicyCommand
	out   domainincident.Policy
	err   error
}

func (m *mockUpsertPolicy) Handle(_ context.Context, cmd cmdincident.UpsertPolicyCommand) (domainincident.Policy, error) {
	m.calls++
	m.got = cmd
	return m.out, m.err
}

type mockList struct {
	calls int
	out   []domainincident.Incident
	total int64
	err   error
}

func (m *mockList) Handle(_ context.Context, _ queryincident.ListIncidentsQuery) ([]domainincident.Incident, int64, error) {
	m.calls++
	return m.out, m.total, m.err
}

type mockGet struct {
	calls int
	out   *queryincident.IncidentDetail
	err   error
}

func (m *mockGet) Handle(_ context.Context, _ queryincident.GetIncidentQuery) (*queryincident.IncidentDetail, error) {
	m.calls++
	return m.out, m.err
}

type mockAck struct {
	calls int
	out   domainincident.Incident
	err   error
}

func (m *mockAck) Handle(_ context.Context, _ cmdincident.AcknowledgeCommand) (domainincident.Incident, error) {
	m.calls++
	return m.out, m.err
}

type mocks struct {
	listPolicies *mockListPolicies
	upsert       *mockUpsertPolicy
	list         *mockList
	get          *mockGet
	ack          *mockAck
}

func newMocks() mocks {
	return mocks{
		listPolicies: &mockListPolicies{},
		upsert:       &mockUpsertPolicy{},
		list:         &mockList{},
		get:          &mockGet{},
		ack:          &mockAck{},
	}
}

func mount(m mocks, auth bool) *fiber.App {
	h := handler.NewIncidentHandler(m.listPolicies, m.upsert, m.list, m.get, m.ack)
	app := testutil.NewTestApp()
	if auth {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows/:id/incident-policies", h.ListPolicies)
	app.Put("/workflows/:id/incident-policies/:kind", h.UpsertPolicy)
	app.Get("/workflows/:id/incidents", h.List)
	app.Get("/incidents/:id", h.GetByID)
	app.Post("/incidents/:id/acknowledge", h.Acknowledge)
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
