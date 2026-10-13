package usertest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	queryuser "go-api/internal/application/query/user"
	domainuser "go-api/internal/domain/user"
	"go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

type mockGetUserByIDHandler struct {
	calls int
	views []*domainuser.UserView
	errs  []error
}

func (m *mockGetUserByIDHandler) Handle(
	_ context.Context,
	q queryuser.GetUserByIDQuery,
) (*domainuser.UserView, error) {
	idx := m.calls
	m.calls++
	if idx >= len(m.errs) {
		return nil, errors.New("unexpected get user call")
	}
	if m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	if q.ID != testutil.TestUserID {
		return nil, errors.New("unexpected user id")
	}
	return m.views[idx], nil
}

func sampleUserView() *domainuser.UserView {
	return &domainuser.UserView{
		ID:        testutil.TestUserID,
		ClerkID:   "clerk_123",
		FirstName: "Jane",
		LastName:  "Doe",
		Email:     "jane@example.com",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestUserHandler_GetUser_Success(t *testing.T) {
	getByID := &mockGetUserByIDHandler{
		views: []*domainuser.UserView{sampleUserView()},
		errs:  []error{nil},
	}
	h := handler.NewUserHandler(getByID)

	app := testutil.NewTestApp()
	app.Get("/user", testutil.WithUserWithoutProject(testutil.TestUserID), h.GetUser)

	req, err := testutil.JSONRequest(http.MethodGet, "/user", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if getByID.calls != 1 {
		t.Fatalf("get user calls: got %d want 1", getByID.calls)
	}

	var out presenter.UserDetailResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testutil.TestUserID.String() {
		t.Fatalf("response id: got %s", out.ID)
	}
	if out.Email != "jane@example.com" {
		t.Fatalf("response email: got %q", out.Email)
	}
}

func TestUserHandler_GetUser_Unauthorized(t *testing.T) {
	getByID := &mockGetUserByIDHandler{}
	h := handler.NewUserHandler(getByID)

	app := testutil.NewTestApp()
	app.Get("/user", h.GetUser)

	req, err := testutil.JSONRequest(http.MethodGet, "/user", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if getByID.calls != 0 {
		t.Fatal("get user handler must not be called without auth")
	}
}

func TestUserHandler_GetUser_HandlerError_Internal(t *testing.T) {
	getByID := &mockGetUserByIDHandler{
		views: []*domainuser.UserView{nil},
		errs:  []error{errors.New("database unavailable")},
	}
	h := handler.NewUserHandler(getByID)

	app := testutil.NewTestApp()
	app.Get("/user", testutil.WithUserWithoutProject(testutil.TestUserID), h.GetUser)

	req, err := testutil.JSONRequest(http.MethodGet, "/user", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
