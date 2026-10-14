package workflowtest

import (
	"context"
	"errors"
	"time"

	schedulecmd "go-api/internal/application/command/schedule"
	workflowcmd "go-api/internal/application/command/workflow"
	queryworkflow "go-api/internal/application/query/workflow"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000a1")
	testRunID      = uuid.MustParse("01960000-0000-7000-8000-0000000000b1")
	errDatabase    = errors.New("pq: connection refused on 10.0.0.12")
)

type mockCreateHandler struct {
	calls int
	got   workflowcmd.CreateWorkflowCommand
	err   error
}

func (m *mockCreateHandler) Handle(_ context.Context, cmd workflowcmd.CreateWorkflowCommand) (*domainworkflow.Workflow, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return &domainworkflow.Workflow{ID: testWorkflowID, UserID: cmd.UserID, Name: cmd.Definition.Name}, nil
}

type mockUpdateHandler struct {
	calls int
	got   workflowcmd.UpdateWorkflowCommand
	err   error
}

func (m *mockUpdateHandler) Handle(_ context.Context, cmd workflowcmd.UpdateWorkflowCommand) (*domainworkflow.Workflow, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return nil, m.err
	}
	return &domainworkflow.Workflow{ID: cmd.WorkflowID, UserID: cmd.UserID, Name: cmd.Definition.Name}, nil
}

type mockListHandler struct {
	calls int
	got   queryworkflow.ListWorkflowsQuery
	views []domainworkflow.ListView
	total int64
	err   error
}

func (m *mockListHandler) Handle(_ context.Context, q queryworkflow.ListWorkflowsQuery) ([]domainworkflow.ListView, int64, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.views, m.total, nil
}

type mockGetHandler struct {
	calls int
	got   queryworkflow.GetWorkflowQuery
	view  *domainworkflow.View
	err   error
}

func (m *mockGetHandler) Handle(_ context.Context, q queryworkflow.GetWorkflowQuery) (*domainworkflow.View, error) {
	m.calls++
	m.got = q
	if m.err != nil {
		return nil, m.err
	}
	return m.view, nil
}

type mockDeleteHandler struct {
	calls int
	got   workflowcmd.DeleteWorkflowCommand
	err   error
}

func (m *mockDeleteHandler) Handle(_ context.Context, cmd workflowcmd.DeleteWorkflowCommand) error {
	m.calls++
	m.got = cmd
	return m.err
}

type mockRunHandler struct {
	calls int
	got   schedulecmd.RunManualCommand
	runID uuid.UUID
	err   error
}

func (m *mockRunHandler) Handle(_ context.Context, cmd schedulecmd.RunManualCommand) (uuid.UUID, error) {
	m.calls++
	m.got = cmd
	if m.err != nil {
		return uuid.Nil, m.err
	}
	if m.runID != uuid.Nil {
		return m.runID, nil
	}
	return testRunID, nil
}

type mocks struct {
	create *mockCreateHandler
	update *mockUpdateHandler
	list   *mockListHandler
	get    *mockGetHandler
	delete *mockDeleteHandler
	run    *mockRunHandler
}

func newMocks() mocks {
	return mocks{
		create: &mockCreateHandler{},
		update: &mockUpdateHandler{},
		list:   &mockListHandler{},
		get:    &mockGetHandler{},
		delete: &mockDeleteHandler{},
		run:    &mockRunHandler{},
	}
}

// mountRoutes registers the workflow routes; authenticated adds a user to the context.
func mountRoutes(m mocks, authenticated bool) *fiber.App {
	h := handler.NewWorkflowHandler(m.create, m.update, m.list, m.get, m.delete, m.run)
	app := testutil.NewTestApp()
	if authenticated {
		app.Use(testutil.WithUserWithoutProject(testutil.TestUserID))
	}
	app.Get("/workflows", h.List)
	app.Post("/workflows", h.Create)
	app.Get("/workflows/:id", h.GetByID)
	app.Put("/workflows/:id", h.Update)
	app.Delete("/workflows/:id", h.Delete)
	app.Post("/workflows/:id/runs", h.Run)
	return app
}

func validWorkflowBody() map[string]any {
	return map[string]any{
		"name":            "Checkout",
		"scheduleType":    "interval",
		"intervalSeconds": 300,
	}
}

func sampleListViews() []domainworkflow.ListView {
	return []domainworkflow.ListView{
		{ID: testWorkflowID, Name: "Checkout"},
		{ID: uuid.MustParse("01960000-0000-7000-8000-0000000000a2"), Name: "Never run"},
	}
}

func sampleView() *domainworkflow.View {
	next := time.Date(2026, 10, 13, 12, 5, 0, 0, time.UTC)
	return &domainworkflow.View{
		ID:              testWorkflowID,
		UserID:          testutil.TestUserID,
		Name:            "Checkout",
		ScheduleType:    domainworkflow.ScheduleInterval,
		IntervalSeconds: 300,
		Enabled:         true,
		Status:          domainschedule.StatusIdle,
		NextRunAt:       &next,
		LastStatus:      "success",
		CreatedAt:       time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	}
}
