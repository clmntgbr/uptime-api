package librarytest

import (
	"net/http"
	"testing"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

func TestLibraryHandler_List_Success(t *testing.T) {
	m := newMocks()
	m.list.views = []domainlibrary.ListView{{ID: testItemID, Name: "Health", Method: "GET", URL: "https://x", CreatedAt: testTime, UpdatedAt: testTime}}
	m.list.total = 1

	resp := doJSON(t, m, true, http.MethodGet, libraryPath+"?page=2&limit=5&search=hea&sortBy=name", nil)
	assertStatus(t, resp, http.StatusOK)

	out := testutil.DecodeJSONMap(t, resp)
	if out["total"] != float64(1) || out["page"] != float64(2) || out["limit"] != float64(5) || out["totalPages"] != float64(1) {
		t.Fatalf("unexpected pagination envelope %v", out)
	}
	if m.list.got.UserID != testutil.TestUserID || m.list.got.Query.Search != "hea" || m.list.got.Query.SortBy != "name" {
		t.Fatalf("unexpected query %+v", m.list.got)
	}
}

func TestLibraryHandler_List_DefaultSortAndEmptyPage(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, libraryPath, nil)
	assertStatus(t, resp, http.StatusOK)
	if m.list.got.Query.SortBy != "createdAt" || m.list.got.Query.OrderBy != "desc" {
		t.Fatalf("default sort not applied: %+v", m.list.got.Query)
	}
	out := testutil.DecodeJSONMap(t, resp)
	if members, ok := out["members"].([]any); !ok || len(members) != 0 {
		t.Fatalf("empty page must serialize as [], got %v", out["members"])
	}
}

func TestLibraryHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, libraryPath, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_List_InvalidQuery(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, libraryPath+"?limit=500", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.list.calls != 0 {
		t.Fatal("list must not run with invalid query")
	}
}

func TestLibraryHandler_List_HandlerError(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, libraryPath, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list library items" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestLibraryHandler_GetByID_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, libraryItem, nil)
	assertStatus(t, resp, http.StatusOK)

	var out presenter.LibraryItemResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.ID != testItemID.String() || out.Name != "Health" || out.Method != "GET" || out.TimeoutMS != 5000 {
		t.Fatalf("unexpected body %+v", out)
	}
	if m.get.got.UserID != testutil.TestUserID || m.get.got.ItemID != testItemID {
		t.Fatalf("unexpected query %+v", m.get.got)
	}
}

func TestLibraryHandler_GetByID_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, libraryItem, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_GetByID_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, libraryPath+"/not-a-uuid", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.get.calls != 0 {
		t.Fatal("get must not run with invalid id")
	}
}

func TestLibraryHandler_GetByID_NotFound(t *testing.T) {
	m := newMocks()
	m.get.err = domainlibrary.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, libraryItem, nil)
	assertStatus(t, resp, http.StatusNotFound)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Library item not found" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestLibraryHandler_GetByID_HandlerError(t *testing.T) {
	m := newMocks()
	m.get.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, libraryItem, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
}

func TestLibraryHandler_Create_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, libraryPath, validBody())
	assertStatus(t, resp, http.StatusCreated)

	var out presenter.LibraryItemResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Name != "Health" || out.URL != "https://api.example.com/health" {
		t.Fatalf("unexpected body %+v", out)
	}
	cmd := m.create.got
	if cmd.UserID != testutil.TestUserID || cmd.Definition.Name != "Health" || len(cmd.Definition.Rules) != 1 {
		t.Fatalf("unexpected command %+v", cmd)
	}
}

func TestLibraryHandler_Create_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, libraryPath, validBody())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_Create_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doRaw(t, m, http.MethodPost, libraryPath, `[]`)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.create.calls != 0 {
		t.Fatal("create must not run on invalid body")
	}
}

func TestLibraryHandler_Create_ValidationFailed(t *testing.T) {
	m := newMocks()
	body := validBody()
	body["name"] = ""
	resp := doJSON(t, m, true, http.MethodPost, libraryPath, body)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.create.calls != 0 {
		t.Fatal("create must not run on invalid payload")
	}
}

func TestLibraryHandler_Create_InvalidRule(t *testing.T) {
	m := newMocks()
	body := validBody()
	body["rules"] = []map[string]any{{"subject": "status", "operator": "exceeds", "value": "200"}}
	resp := doJSON(t, m, true, http.MethodPost, libraryPath, body)
	assertStatus(t, resp, http.StatusBadRequest)
	errs, _ := testutil.DecodeJSONMap(t, resp)["errors"].(map[string]any)
	if _, ok := errs["rules[0]"]; !ok {
		t.Fatalf("expected rules[0] error, got %v", errs)
	}
}

func TestLibraryHandler_Create_HandlerError_InvalidCall(t *testing.T) {
	m := newMocks()
	m.create.err = &domaincall.ValidationError{Call: "Health", Message: "url is required"}
	resp := doJSON(t, m, true, http.MethodPost, libraryPath, validBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["code"] != "INVALID_CALL" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestLibraryHandler_Create_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.create.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, libraryPath, validBody())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to create library item" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestLibraryHandler_Update_Success(t *testing.T) {
	m := newMocks()
	body := validBody()
	body["name"] = "Renamed"
	resp := doJSON(t, m, true, http.MethodPut, libraryItem, body)
	assertStatus(t, resp, http.StatusOK)
	if m.update.got.ItemID != testItemID || m.update.got.UserID != testutil.TestUserID || m.update.got.Definition.Name != "Renamed" {
		t.Fatalf("unexpected command %+v", m.update.got)
	}
}

func TestLibraryHandler_Update_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPut, libraryItem, validBody())
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_Update_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, libraryPath+"/nope", validBody())
	assertStatus(t, resp, http.StatusBadRequest)
	if m.update.calls != 0 {
		t.Fatal("update must not run with invalid id")
	}
}

func TestLibraryHandler_Update_ValidationFailed(t *testing.T) {
	m := newMocks()
	body := validBody()
	delete(body, "url")
	resp := doJSON(t, m, true, http.MethodPut, libraryItem, body)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.update.calls != 0 {
		t.Fatal("update must not run on invalid payload")
	}
}

func TestLibraryHandler_Update_NotFound(t *testing.T) {
	m := newMocks()
	m.update.err = domainlibrary.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPut, libraryItem, validBody())
	assertStatus(t, resp, http.StatusNotFound)
}

func TestLibraryHandler_Update_HandlerError_InvalidCall(t *testing.T) {
	m := newMocks()
	m.update.err = &domaincall.ValidationError{Message: "timeout must be between"}
	resp := doJSON(t, m, true, http.MethodPut, libraryItem, validBody())
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestLibraryHandler_Update_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.update.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPut, libraryItem, validBody())
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to update library item" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestLibraryHandler_Delete_Success(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, libraryItem, nil)
	assertStatus(t, resp, http.StatusNoContent)
	if m.delete.got.ItemID != testItemID || m.delete.got.UserID != testutil.TestUserID {
		t.Fatalf("unexpected command %+v", m.delete.got)
	}
}

func TestLibraryHandler_Delete_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodDelete, libraryItem, nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestLibraryHandler_Delete_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodDelete, libraryPath+"/nope", nil)
	assertStatus(t, resp, http.StatusBadRequest)
	if m.delete.calls != 0 {
		t.Fatal("delete must not run with invalid id")
	}
}

func TestLibraryHandler_Delete_NotFound(t *testing.T) {
	m := newMocks()
	m.delete.err = domainlibrary.ErrNotFound
	resp := doJSON(t, m, true, http.MethodDelete, libraryItem, nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestLibraryHandler_Delete_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.delete.err = errDatabase
	resp := doJSON(t, m, true, http.MethodDelete, libraryItem, nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to delete library item" {
		t.Fatalf("unexpected body %v", out)
	}
}
