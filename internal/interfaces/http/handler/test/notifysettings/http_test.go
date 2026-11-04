package notifysettingstest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	cmdnotify "go-api/internal/application/command/notifysettings"
	querynotify "go-api/internal/application/query/notifysettings"
	domainnotify "go-api/internal/domain/notifysettings"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	testUserID     = testutil.TestUserID
	testWorkflowID = uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	errInternal    = errors.New("db down")
)

type mockGet struct {
	settings domainnotify.Settings
	err      error
}

func (m *mockGet) Handle(_ context.Context, q querynotify.GetQuery) (domainnotify.Settings, error) {
	if m.err != nil {
		return domainnotify.Settings{}, m.err
	}
	return m.settings, nil
}

type mockUpsert struct {
	got cmdnotify.UpsertCommand
	err error
}

func (m *mockUpsert) Handle(_ context.Context, cmd cmdnotify.UpsertCommand) (domainnotify.Settings, error) {
	m.got = cmd
	if m.err != nil {
		return domainnotify.Settings{}, m.err
	}
	return domainnotify.Settings{
		WorkflowID: cmd.WorkflowID, UserID: cmd.UserID, Emails: cmd.Emails,
		IncidentsEnabled: cmd.IncidentsEnabled, TLSEnabled: cmd.TLSEnabled,
		RunsEnabled: cmd.RunsEnabled, RunMode: domainnotify.RunModeTransition,
	}, nil
}

func mount(t *testing.T, get *mockGet, upsert *mockUpsert, withUser bool) *fiber.App {
	t.Helper()
	app := testutil.NewTestApp()
	h := handler.NewNotifySettingsHandler(get, upsert)
	var group fiber.Router = app.Group("/api")
	if withUser {
		group = app.Group("/api", testutil.WithActiveProject(testUserID, testutil.TestProjectID))
	}
	group.Get("/workflows/:id/notification-settings", h.Get)
	group.Put("/workflows/:id/notification-settings", h.Upsert)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any) *http.Response {
	t.Helper()
	req, err := testutil.JSONRequest(method, path, body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestNotifySettingsHandler_Get_Success(t *testing.T) {
	get := &mockGet{settings: domainnotify.DefaultSettings(testUserID, testWorkflowID, "a@b.com")}
	app := mount(t, get, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodGet, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	out := testutil.DecodeJSONMap(t, resp)
	emails, _ := out["emails"].([]any)
	if len(emails) != 1 || emails[0] != "a@b.com" {
		t.Fatalf("emails %#v", out["emails"])
	}
}

func TestNotifySettingsHandler_Get_Unauthorized(t *testing.T) {
	app := mount(t, &mockGet{}, &mockUpsert{}, false)
	resp := doJSON(t, app, http.MethodGet, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Get_NotFound(t *testing.T) {
	get := &mockGet{err: domainworkflow.ErrNotFound}
	app := mount(t, get, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodGet, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Get_InternalError(t *testing.T) {
	get := &mockGet{err: errInternal}
	app := mount(t, get, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodGet, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Get_BadWorkflowID(t *testing.T) {
	app := mount(t, &mockGet{}, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodGet, "/api/workflows/bad/notification-settings", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Upsert_Success(t *testing.T) {
	up := &mockUpsert{}
	app := mount(t, &mockGet{}, up, true)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", map[string]any{
		"emails": []string{"a@b.com"}, "incidentsEnabled": true, "tlsEnabled": true,
		"runsEnabled": false, "runStatuses": []string{}, "runMode": "transition",
	})
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	if len(up.got.Emails) != 1 || up.got.Emails[0] != "a@b.com" {
		t.Fatalf("got %+v", up.got)
	}
}

func TestNotifySettingsHandler_Upsert_Unauthorized(t *testing.T) {
	app := mount(t, &mockGet{}, &mockUpsert{}, false)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", map[string]any{
		"emails": []string{"a@b.com"}, "incidentsEnabled": true, "tlsEnabled": true,
		"runsEnabled": false, "runStatuses": []string{}, "runMode": "transition",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Upsert_Invalid(t *testing.T) {
	up := &mockUpsert{err: domainnotify.ErrInvalid}
	app := mount(t, &mockGet{}, up, true)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", map[string]any{
		"emails": []string{"bad"}, "incidentsEnabled": true, "tlsEnabled": true,
		"runsEnabled": false, "runStatuses": []string{}, "runMode": "transition",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Upsert_BadWorkflowID(t *testing.T) {
	app := mount(t, &mockGet{}, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/bad/notification-settings", map[string]any{
		"emails": []string{"a@b.com"}, "runMode": "transition",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Upsert_InternalError(t *testing.T) {
	up := &mockUpsert{err: errInternal}
	app := mount(t, &mockGet{}, up, true)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", map[string]any{
		"emails": []string{"a@b.com"}, "incidentsEnabled": true, "tlsEnabled": true,
		"runsEnabled": false, "runStatuses": []string{}, "runMode": "transition",
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNotifySettingsHandler_Upsert_ValidationFailed(t *testing.T) {
	app := mount(t, &mockGet{}, &mockUpsert{}, true)
	resp := doJSON(t, app, http.MethodPut, "/api/workflows/"+testWorkflowID.String()+"/notification-settings", map[string]any{
		"emails": []string{"a@b.com"}, "runMode": "never",
	})
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}
