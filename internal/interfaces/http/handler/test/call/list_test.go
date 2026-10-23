package calltest

import (
	"net/http"
	"testing"
	"time"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func TestCallHandler_List_Success(t *testing.T) {
	m := newMocks()
	httpStatus := 200
	latency := int64(142)
	olderLatency := int64(90)
	finished := time.Date(2026, 10, 19, 14, 45, 0, 0, time.UTC)
	older := finished.Add(-time.Minute)
	runID := uuid.MustParse("01960000-0000-7000-8000-0000000000d1")
	callRunID := uuid.MustParse("01960000-0000-7000-8000-0000000000d2")
	m.list.views = []domainworkflow.CallListView{
		{
			ID: testCallID, Name: "login", Method: "POST", URL: "https://api.example.com/auth",
			Position: 1, LastStatus: "success", LastHTTPStatus: &httpStatus,
			LastFinishedAt: &finished, LastLatencyMs: &latency,
			RecentRuns: []domainworkflow.CallRecentRunView{
				{ID: callRunID, RunID: runID, Status: "failed", FinishedAt: &older, LatencyMs: &olderLatency},
				{ID: callRunID, RunID: runID, Status: "success", HTTPStatus: &httpStatus, FinishedAt: &finished, LatencyMs: &latency},
			},
		},
		{
			ID: uuid.MustParse("01960000-0000-7000-8000-0000000000c2"), Name: "orders",
			Method: "GET", URL: "https://api.example.com/orders", Position: 2,
			RecentRuns: []domainworkflow.CallRecentRunView{},
		},
	}
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusOK)

	if m.list.got.UserID != testutil.TestUserID || m.list.got.WorkflowID != testWorkflowID {
		t.Fatalf("unexpected query %+v", m.list.got)
	}
	var out []presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 2 ||
		out[0].ID != testCallID.String() || out[0].Name != "login" ||
		out[0].Method != "POST" || out[0].URL != "https://api.example.com/auth" ||
		out[0].Position != 1 || out[0].LastStatus == nil || *out[0].LastStatus != "success" ||
		out[0].LastHTTPStatus == nil || *out[0].LastHTTPStatus != 200 ||
		out[0].LastFinishedAt == nil || *out[0].LastFinishedAt != "2026-10-19T14:45:00Z" ||
		out[0].LastLatencyMs == nil || *out[0].LastLatencyMs != 142 ||
		len(out[0].RecentRuns) != 2 || out[0].RecentRuns[0].Status != "failed" ||
		out[0].RecentRuns[1].Status != "success" || out[0].RecentRuns[1].RunID != runID.String() ||
		out[1].Method != "GET" || out[1].URL != "https://api.example.com/orders" ||
		out[1].LastStatus != nil || out[1].LastHTTPStatus != nil ||
		out[1].LastFinishedAt != nil || out[1].LastLatencyMs != nil ||
		out[1].RecentRuns == nil || len(out[1].RecentRuns) != 0 {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestCallHandler_List_Empty(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusOK)
	var out []presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if out == nil || len(out) != 0 {
		t.Fatalf("empty list must be [], got %#v", out)
	}
}

func TestCallHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_List_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/not-a-uuid/calls", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestCallHandler_List_HandlerError_NotFound(t *testing.T) {
	m := newMocks()
	m.list.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_List_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, callsPath(), nil)
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to list calls" {
		t.Fatalf("unexpected body %v", out)
	}
}
