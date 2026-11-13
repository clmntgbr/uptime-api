package librarytest

import (
	"net/http"
	"testing"

	cmdlibrary "go-api/internal/application/command/library"
	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func fromCallPath() string {
	return libraryPath + "/from-call/" + testCallID.String()
}

func TestLibraryHandler_SaveFromCall_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, fromCallPath(), nil)
	assertStatus(t, resp, http.StatusCreated)

	var out presenter.LibraryItemResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testItemID.String() || out.Name != "Health" {
		t.Fatalf("unexpected body %+v", out)
	}
	if m.saveFromCall.got != (cmdlibrary.SaveFromCallCommand{UserID: testutil.TestUserID, CallID: testCallID}) {
		t.Fatalf("unexpected command %+v", m.saveFromCall.got)
	}
}

func TestLibraryHandler_SaveFromCall_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, fromCallPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_SaveFromCall_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, libraryPath+"/from-call/nope", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.saveFromCall.calls != 0 {
		t.Fatal("save must not run with invalid call id")
	}
}

func TestLibraryHandler_SaveFromCall_HandlerErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"call not found or not owned", domainworkflow.ErrCallNotFound, http.StatusNotFound, "Call not found"},
		{"invalid call", &domaincall.ValidationError{Message: "x"}, http.StatusBadRequest, ""},
		{"item not found", domainlibrary.ErrNotFound, http.StatusNotFound, "Library item not found"},
		{"internal", errDatabase, http.StatusInternalServerError, "Failed to save call to library"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMocks()
			m.saveFromCall.err = tc.err
			resp := doJSON(t, m, true, http.MethodPost, fromCallPath(), nil)
			assertStatus(t, resp, tc.status)
			if out := testutil.DecodeJSONMap(t, resp); tc.message != "" && out["message"] != tc.message {
				t.Fatalf("message: got %v want %q", out["message"], tc.message)
			}
		})
	}
}
