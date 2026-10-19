package runtest

import (
	"net/http"
	"testing"

	"go-api/internal/domain/paginate"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"
)

var listPath = "/workflows/" + testWorkflowID.String() + "/runs"

type listResponse struct {
	Total   int                             `json:"total"`
	Members []presenter.RunListItemResponse `json:"members"`
}

func TestRunHandler_List_Success(t *testing.T) {
	m := newMocks()
	m.list.views = sampleListViews()
	m.list.total = 1

	resp := doJSON(t, m, true, http.MethodGet, listPath)
	assertStatus(t, resp, http.StatusOK)

	if m.list.calls != 1 || m.list.got.UserID != testutil.TestUserID || m.list.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.list.got)
	}
	if m.list.got.Query.SortBy != "startedAt" || m.list.got.Query.OrderBy != paginate.OrderByDesc {
		t.Fatalf("unexpected pagination defaults %+v", m.list.got.Query)
	}

	var out listResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Total != 1 || len(out.Members) != 1 || out.Members[0].ID != testRunID.String() {
		t.Fatalf("unexpected body %+v", out)
	}
	if out.Members[0].Status != "success" || out.Members[0].Counts.Total != 2 {
		t.Fatalf("unexpected member %+v", out.Members[0])
	}
}

func TestRunHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, listPath)
	assertStatus(t, resp, http.StatusUnauthorized)
	if m.list.calls != 0 {
		t.Fatal("list must not be called without a user")
	}
}

func TestRunHandler_List_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/not-a-uuid/runs")
	assertStatus(t, resp, http.StatusBadRequest)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Invalid workflow id" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestRunHandler_List_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, listPath)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list runs" {
		t.Fatalf("unexpected body %v", out)
	}
}
