package workflowtest

import (
	"net/http"
	"testing"

	"go-api/internal/domain/paginate"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

type listResponse struct {
	Total      int                                  `json:"total"`
	Page       int                                  `json:"page"`
	Limit      int                                  `json:"limit"`
	TotalPages int                                  `json:"totalPages"`
	Members    []presenter.WorkflowListItemResponse `json:"members"`
}

func TestWorkflowHandler_List_Success(t *testing.T) {
	m := newMocks()
	m.list.views = sampleListViews()
	m.list.total = 45

	resp := doJSON(t, m, true, http.MethodGet, "/workflows", nil)
	assertStatus(t, resp, http.StatusOK)

	got := m.list.got
	if got.UserID != testutil.TestUserID || got.Query.Page != 1 || got.Query.Limit != paginate.DefaultLimit ||
		got.Query.SortBy != "createdAt" || got.Query.OrderBy != paginate.OrderByDesc {
		t.Fatalf("defaults must be applied before the query: %+v", got)
	}

	var out listResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Total != 45 || out.TotalPages != 3 || out.Page != 1 || len(out.Members) != 2 {
		t.Fatalf("unexpected page %+v", out)
	}
	if first := out.Members[0]; first.ID != testWorkflowID.String() || first.Name != "Checkout" {
		t.Fatalf("unexpected first member %+v", first)
	}
}

func TestWorkflowHandler_List_OnlyIDAndName(t *testing.T) {
	m := newMocks()
	m.list.views = sampleListViews()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows", nil)
	assertStatus(t, resp, http.StatusOK)

	members, _ := testutil.DecodeJSONMap(t, resp)["members"].([]any)
	for _, member := range members {
		if item, _ := member.(map[string]any); len(item) != 2 || item["id"] == nil || item["name"] == nil {
			t.Fatalf("a list item must only expose id and name, got %v", member)
		}
	}
}

func TestWorkflowHandler_List_QueryParameters(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows?page=2&limit=5&sortBy=name&orderBy=asc&search=check", nil)
	assertStatus(t, resp, http.StatusOK)

	q := m.list.got.Query
	if q.Page != 2 || q.Limit != 5 || q.SortBy != "name" || q.OrderBy != paginate.OrderByAsc || q.Search != "check" {
		t.Fatalf("query parameters must be passed through: %+v", q)
	}
}

func TestWorkflowHandler_List_EmptyPage(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows", nil)
	assertStatus(t, resp, http.StatusOK)

	out := testutil.DecodeJSONMap(t, resp)
	members, ok := out["members"].([]any)
	if !ok || len(members) != 0 {
		t.Fatalf("an empty page must serialize members as [], got %v", out["members"])
	}
}

func TestWorkflowHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, "/workflows", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.list.calls != 0 {
		t.Fatal("list must not be called without a user")
	}
}

func TestWorkflowHandler_List_InvalidQuery(t *testing.T) {
	for _, query := range []string{"limit=1000", "orderBy=sideways", "page=abc"} {
		t.Run(query, func(t *testing.T) {
			m := newMocks()
			resp := doJSON(t, m, true, http.MethodGet, "/workflows?"+query, nil)
			assertStatus(t, resp, http.StatusBadRequest)
			if m.list.calls != 0 {
				t.Fatal("list must not be called on invalid query parameters")
			}
		})
	}
}

func TestWorkflowHandler_List_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, "/workflows", nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list workflows" {
		t.Fatalf("internal errors must not leak, got %v", out)
	}
}
