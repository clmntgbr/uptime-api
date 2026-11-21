package feedtest

import (
	"net/http"
	"testing"
	"time"

	queryfeed "go-api/internal/application/query/feed"
	domainfeed "go-api/internal/domain/feed"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func TestFeedHandler_List_Success(t *testing.T) {
	list := &mockList{out: queryfeed.ListFeedResult{
		Events:     []domainfeed.Event{sampleEvent()},
		NextCursor: "abc",
	}}
	wf := testutil.TestProjectID
	resp := doJSON(t, list, true, "/feed?workflowId="+wf.String()+"&limit=10&kind=call.failed&severity=critical")
	assertStatus(t, resp, http.StatusOK)
	var out presenter.FeedListResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Members) != 1 || out.Members[0].Kind != "call.failed" || out.NextCursor != "abc" {
		t.Fatalf("unexpected %+v", out)
	}
	if list.got.UserID != testutil.TestUserID || list.got.WorkflowID == nil || *list.got.WorkflowID != wf {
		t.Fatalf("unexpected query %+v", list.got)
	}
	if list.got.Limit != 10 || len(list.got.Kinds) != 1 || list.got.Kinds[0] != domainfeed.KindCallFailed {
		t.Fatalf("unexpected filters %+v", list.got)
	}
}

func TestFeedHandler_List_Unauthorized(t *testing.T) {
	resp := doJSON(t, &mockList{}, false, "/feed")
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestFeedHandler_List_InvalidWorkflowID(t *testing.T) {
	resp := doJSON(t, &mockList{}, true, "/feed?workflowId=bad")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestFeedHandler_List_InvalidFrom(t *testing.T) {
	resp := doJSON(t, &mockList{}, true, "/feed?from=not-a-date")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestFeedHandler_List_InvalidCursor(t *testing.T) {
	list := &mockList{err: queryfeed.ErrInvalidCursor}
	resp := doJSON(t, list, true, "/feed?cursor=nope")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestFeedHandler_List_HandlerError(t *testing.T) {
	list := &mockList{err: errDatabase}
	resp := doJSON(t, list, true, "/feed")
	assertStatus(t, resp, http.StatusInternalServerError)
}

func TestFeedHandler_List_WithTimeRange(t *testing.T) {
	list := &mockList{out: queryfeed.ListFeedResult{Events: []domainfeed.Event{}}}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	resp := doJSON(t, list, true, "/feed?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339))
	assertStatus(t, resp, http.StatusOK)
	if list.got.From == nil || !list.got.From.Equal(from) || list.got.To == nil || !list.got.To.Equal(to) {
		t.Fatalf("unexpected range from=%v to=%v", list.got.From, list.got.To)
	}
}

func TestFeedHandler_List_EmptyMembers(t *testing.T) {
	list := &mockList{out: queryfeed.ListFeedResult{Events: nil}}
	resp := doJSON(t, list, true, "/feed")
	assertStatus(t, resp, http.StatusOK)
	var out presenter.FeedListResponse
	testutil.DecodeJSON(t, resp, &out)
	if out.Members == nil || len(out.Members) != 0 {
		t.Fatalf("members must be [], got %#v", out.Members)
	}
}

func TestFeedHandler_List_InvalidTo(t *testing.T) {
	resp := doJSON(t, &mockList{}, true, "/feed?to=bad")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestFeedHandler_List_PassesCursor(t *testing.T) {
	list := &mockList{out: queryfeed.ListFeedResult{}}
	_ = uuid.New()
	resp := doJSON(t, list, true, "/feed?cursor=abc123")
	assertStatus(t, resp, http.StatusOK)
	if list.got.Cursor != "abc123" {
		t.Fatalf("cursor=%q", list.got.Cursor)
	}
}
