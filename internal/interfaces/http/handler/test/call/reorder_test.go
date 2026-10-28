package calltest

import (
	"net/http"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func reorderPath() string {
	return callsPath() + "/reorder"
}

func TestCallHandler_Reorder_Success(t *testing.T) {
	idA := uuid.MustParse("01960000-0000-7000-8000-0000000000c1")
	idB := uuid.MustParse("01960000-0000-7000-8000-0000000000c2")
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{idB.String(), idA.String()},
	})
	assertStatus(t, resp, http.StatusOK)

	if m.reorder.calls != 1 || m.reorder.got.UserID != testutil.TestUserID ||
		m.reorder.got.WorkflowID != testWorkflowID ||
		len(m.reorder.got.CallIDs) != 2 ||
		m.reorder.got.CallIDs[0] != idB || m.reorder.got.CallIDs[1] != idA {
		t.Fatalf("unexpected command %+v calls=%d", m.reorder.got, m.reorder.calls)
	}
	var out []presenter.CallResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out) != 2 || out[0].ID != idB.String() || out[0].Position != 1 ||
		out[1].ID != idA.String() || out[1].Position != 2 {
		t.Fatalf("unexpected body %+v", out)
	}
}

func TestCallHandler_Reorder_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestCallHandler_Reorder_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/not-a-uuid/calls/reorder", map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusBadRequest)
	if m.reorder.calls != 0 {
		t.Fatalf("handler must not be called")
	}
}

func TestCallHandler_Reorder_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{"not-a-uuid"},
	})
	assertStatus(t, resp, http.StatusBadRequest)
	if m.reorder.calls != 0 {
		t.Fatalf("handler must not be called")
	}
}

func TestCallHandler_Reorder_HandlerError_InvalidCallOrder(t *testing.T) {
	m := newMocks()
	m.reorder.err = domainworkflow.ErrInvalidCallOrder
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	if out := testutil.DecodeJSONMap(t, resp); out["code"] != "INVALID_CALL_ORDER" {
		t.Fatalf("unexpected body %v", out)
	}
}

func TestCallHandler_Reorder_HandlerError_WorkflowNotFound(t *testing.T) {
	m := newMocks()
	m.reorder.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestCallHandler_Reorder_HandlerError_VarOutOfOrder(t *testing.T) {
	m := newMocks()
	m.reorder.err = &domainworkflow.ValidationError{
		Call:         "Order call",
		Variable:     "orderId",
		ConflictCall: "Orders call",
		Reason:       domainworkflow.ReasonProducedByLaterCall,
		Message:      `produced by call "Orders call", which runs after it`,
		Kind:         domainworkflow.ErrVarOutOfOrder,
	}
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusBadRequest)
	out := testutil.DecodeJSONMap(t, resp)
	if out["code"] != "VAR_OUT_OF_ORDER" {
		t.Fatalf("unexpected body %v", out)
	}
	data, _ := out["data"].(map[string]any)
	if data["call"] != "Order call" || data["variable"] != "orderId" ||
		data["conflictCall"] != "Orders call" || data["reason"] != "PRODUCED_BY_LATER_CALL" {
		t.Fatalf("unexpected data %v", data)
	}
}

func TestCallHandler_Reorder_HandlerError_Internal(t *testing.T) {
	m := newMocks()
	m.reorder.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPut, reorderPath(), map[string]any{
		"callIds": []string{testCallID.String()},
	})
	assertStatus(t, resp, http.StatusInternalServerError)
	if out := testutil.DecodeJSONMap(t, resp); out["message"] != "Failed to reorder calls" {
		t.Fatalf("unexpected body %v", out)
	}
}
