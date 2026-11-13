package calltest

import (
	"net/http"
	"testing"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func insertPath() string {
	return "/workflows/" + testWorkflowID.String() + "/calls/from-library/" + testItemID.String()
}

func TestCallHandler_InsertFromLibrary_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, insertPath(), map[string]any{"position": 2})
	assertStatus(t, resp, http.StatusCreated)

	var out presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Name != "Health" || out.Position != 2 {
		t.Fatalf("unexpected body %+v", out)
	}
	cmd := m.insertFromLibrary.got
	if cmd.UserID != testutil.TestUserID || cmd.WorkflowID != testWorkflowID || cmd.ItemID != testItemID || cmd.Position != 2 {
		t.Fatalf("unexpected command %+v", cmd)
	}
}

func TestCallHandler_InsertFromLibrary_AppendsWithoutBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, insertPath(), "")
	assertStatus(t, resp, http.StatusCreated)
	if m.insertFromLibrary.got.Position != 0 {
		t.Fatalf("empty body must append (position 0), got %d", m.insertFromLibrary.got.Position)
	}
}

func TestCallHandler_InsertFromLibrary_EmptyObjectAppends(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, insertPath(), map[string]any{})
	assertStatus(t, resp, http.StatusCreated)
	if m.insertFromLibrary.got.Position != 0 {
		t.Fatalf("body without position must append, got %d", m.insertFromLibrary.got.Position)
	}
}

func TestCallHandler_InsertFromLibrary_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, insertPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_InsertFromLibrary_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows/nope/calls/from-library/"+testItemID.String(), nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.insertFromLibrary.calls != 0 {
		t.Fatal("insert must not run with invalid workflow id")
	}
}

func TestCallHandler_InsertFromLibrary_InvalidItemID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/workflows/"+testWorkflowID.String()+"/calls/from-library/nope", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.insertFromLibrary.calls != 0 {
		t.Fatal("insert must not run with invalid item id")
	}
}

func TestCallHandler_InsertFromLibrary_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, insertPath(), `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.insertFromLibrary.calls != 0 {
		t.Fatal("insert must not run on invalid body")
	}
}

func TestCallHandler_InsertFromLibrary_PositionBelowOne(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, insertPath(), map[string]any{"position": 0})
	assertStatus(t, resp, http.StatusBadRequest)
	if m.insertFromLibrary.calls != 0 {
		t.Fatal("insert must not run with position 0")
	}
}

func TestCallHandler_InsertFromLibrary_HandlerErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"item not found", domainlibrary.ErrNotFound, http.StatusNotFound, "Library item not found"},
		{"workflow not found", domainworkflow.ErrNotFound, http.StatusNotFound, "Workflow not found"},
		{"workflow wrong owner", domainworkflow.ErrWrongOwner, http.StatusNotFound, "Workflow not found"},
		{"workflow validation", &domainworkflow.ValidationError{Message: "position must be between 1 and 3"}, http.StatusBadRequest, ""},
		{"invalid call", &domaincall.ValidationError{Call: "Health", Message: "url is required"}, http.StatusBadRequest, ""},
		{"internal", errDatabase, http.StatusInternalServerError, "Failed to insert library item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMocks()
			m.insertFromLibrary.err = tc.err
			resp := doJSON(t, m, true, http.MethodPost, insertPath(), nil)
			assertStatus(t, resp, tc.status)
			out := testutil.DecodeJSONMap(t, resp)
			if tc.message != "" && out["message"] != tc.message {
				t.Fatalf("message: got %v want %q", out["message"], tc.message)
			}
		})
	}
}
