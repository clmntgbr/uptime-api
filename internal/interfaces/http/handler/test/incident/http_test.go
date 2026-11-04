package incidenttest

import (
	"net/http"
	"testing"
	"time"

	domainincident "go-api/internal/domain/incident"
	domainworkflow "go-api/internal/domain/workflow"
	queryincident "go-api/internal/application/query/incident"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func TestIncidentHandler_ListPolicies_Success(t *testing.T) {
	m := newMocks()
	m.listPolicies.out = []domainincident.Policy{
		domainincident.DefaultPolicy(testutil.TestUserID, testWorkflowID, domainincident.KindCallFailed),
	}
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incident-policies", nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.IncidentPoliciesResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Policies) != 1 || out.Policies[0].Kind != "call_failed" {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestIncidentHandler_ListPolicies_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incident-policies", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestIncidentHandler_ListPolicies_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/bad/incident-policies", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_ListPolicies_NotFound(t *testing.T) {
	m := newMocks()
	m.listPolicies.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incident-policies", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestIncidentHandler_UpsertPolicy_Success(t *testing.T) {
	m := newMocks()
	m.upsert.out = domainincident.Policy{
		ID: uuid.New(), Kind: domainincident.KindCallFailed, Enabled: true,
		Severity: domainincident.SeverityCritical, OpenAfter: 1, CloseAfter: 2,
		UpdatedAt: time.Now().UTC(),
	}
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/call_failed", map[string]any{
		"enabled": true, "severity": "critical", "openAfter": 1, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusOK)
	if m.upsert.got.OpenAfter != 1 || m.upsert.got.Kind != domainincident.KindCallFailed {
		t.Fatalf("unexpected %+v", m.upsert.got)
	}
}

func TestIncidentHandler_UpsertPolicy_UnknownKind(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/nope", map[string]any{
		"enabled": true, "severity": "critical", "openAfter": 3, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestIncidentHandler_UpsertPolicy_InvalidBody(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/call_failed", map[string]any{
		"enabled": true, "severity": "nope", "openAfter": 3, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_UpsertPolicy_HandlerInvalid(t *testing.T) {
	m := newMocks()
	m.upsert.err = domainincident.ErrInvalidPolicy
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/availability_low", map[string]any{
		"enabled": true, "severity": "warning", "openAfter": 1, "closeAfter": 1,
		"params": map[string]any{"threshold": 99, "recoveryThreshold": 90},
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestIncidentHandler_List_Success(t *testing.T) {
	m := newMocks()
	m.list.out = []domainincident.Incident{{
		ID: testIncidentID, Kind: domainincident.KindCallFailed,
		Severity: domainincident.SeverityCritical, State: domainincident.StateOpen,
		OpenedAt: time.Now().UTC(),
	}}
	m.list.total = 1
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents?state=open", nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestIncidentHandler_List_InvalidState(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents?state=weird", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_GetByID_Success(t *testing.T) {
	m := newMocks()
	m.get.out = &queryincident.IncidentDetail{
		Incident: domainincident.Incident{
			ID: testIncidentID, WorkflowID: testWorkflowID,
			Kind: domainincident.KindCallFailed, State: domainincident.StateOpen,
			Severity: domainincident.SeverityCritical, OpenedAt: time.Now().UTC(),
			Cause: map[string]any{},
		},
	}
	resp := doJSON(t, m, true, http.MethodGet, "/incidents/"+testIncidentID.String(), nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestIncidentHandler_GetByID_NotFound(t *testing.T) {
	m := newMocks()
	m.get.err = domainincident.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, "/incidents/"+testIncidentID.String(), nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestIncidentHandler_Acknowledge_Success(t *testing.T) {
	m := newMocks()
	m.ack.out = domainincident.Incident{
		ID: testIncidentID, State: domainincident.StateAcknowledged,
		Kind: domainincident.KindCallFailed, Severity: domainincident.SeverityCritical,
		OpenedAt: time.Now().UTC(), Cause: map[string]any{},
	}
	resp := doJSON(t, m, true, http.MethodPost, "/incidents/"+testIncidentID.String()+"/acknowledge", nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestIncidentHandler_Acknowledge_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPost, "/incidents/"+testIncidentID.String()+"/acknowledge", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestIncidentHandler_Acknowledge_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPost, "/incidents/bad/acknowledge", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_List_Internal(t *testing.T) {
	m := newMocks()
	m.list.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents", nil)
	assertStatus(t, resp, http.StatusInternalServerError)
}

func TestIncidentHandler_UpsertPolicy_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/call_failed", map[string]any{
		"enabled": true, "severity": "critical", "openAfter": 3, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestIncidentHandler_UpsertPolicy_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/bad/incident-policies/call_failed", map[string]any{
		"enabled": true, "severity": "critical", "openAfter": 3, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_List_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestIncidentHandler_List_InvalidWorkflowID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/bad/incidents", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_List_SeverityFilter(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents?severity=critical,warning", nil)
	assertStatus(t, resp, http.StatusOK)
}

func TestIncidentHandler_List_InvalidSeverity(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/incidents?severity=loud", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_GetByID_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, "/incidents/"+testIncidentID.String(), nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestIncidentHandler_GetByID_InvalidID(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodGet, "/incidents/bad", nil)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestIncidentHandler_GetByID_Internal(t *testing.T) {
	m := newMocks()
	m.get.err = errDatabase
	resp := doJSON(t, m, true, http.MethodGet, "/incidents/"+testIncidentID.String(), nil)
	assertStatus(t, resp, http.StatusInternalServerError)
}

func TestIncidentHandler_Acknowledge_Internal(t *testing.T) {
	m := newMocks()
	m.ack.err = errDatabase
	resp := doJSON(t, m, true, http.MethodPost, "/incidents/"+testIncidentID.String()+"/acknowledge", nil)
	assertStatus(t, resp, http.StatusInternalServerError)
}

func TestIncidentHandler_UpsertPolicy_UnknownKindError(t *testing.T) {
	m := newMocks()
	m.upsert.err = domainincident.ErrUnknownKind
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/incident-policies/call_failed", map[string]any{
		"enabled": true, "severity": "critical", "openAfter": 3, "closeAfter": 2,
	})
	assertStatus(t, resp, http.StatusNotFound)
}
