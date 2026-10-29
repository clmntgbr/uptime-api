package expirytest

import (
	"net/http"
	"testing"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/testutil"

	"github.com/google/uuid"
)

func TestExpiryHandler_ListPolicies_Success(t *testing.T) {
	m := newMocks()
	m.listPolicies.out = []domainexpiry.Policy{
		domainexpiry.DefaultPolicy(testutil.TestUserID, testWorkflowID, domainexpiry.TypeTLSCertificate),
	}
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/expiry-policies", nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.ExpiryPoliciesResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Policies) != 1 || out.Policies[0].Type != "tls_certificate" || out.Policies[0].Enabled || len(out.Policies[0].ThresholdsDays) != 0 {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestExpiryHandler_UpsertPolicy_Success(t *testing.T) {
	m := newMocks()
	m.upsert.out = domainexpiry.Policy{
		ID: uuid.New(), Type: domainexpiry.TypeTLSCertificate, Enabled: true,
		ThresholdsDays: []int{60, 30, 14}, UpdatedAt: time.Now().UTC(),
	}
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/expiry-policies/tls_certificate", map[string]any{
		"enabled": true, "thresholdsDays": []int{60, 30, 14},
	})
	assertStatus(t, resp, http.StatusOK)
	if m.upsert.got.Enabled != true || len(m.upsert.got.ThresholdsDays) != 3 {
		t.Fatalf("unexpected command %+v", m.upsert.got)
	}
}

func TestExpiryHandler_UpsertPolicy_EmptyThresholdsEnabled(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/expiry-policies/tls_certificate", map[string]any{
		"enabled": true, "thresholdsDays": []int{},
	})
	assertStatus(t, resp, http.StatusUnprocessableEntity)
	if m.upsert.calls != 0 {
		t.Fatal("must not call handler")
	}
}

func TestExpiryHandler_UpsertPolicy_UnknownType(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, true, http.MethodPut, "/workflows/"+testWorkflowID.String()+"/expiry-policies/domain", map[string]any{
		"enabled": false,
	})
	assertStatus(t, resp, http.StatusNotFound)
}

func TestExpiryHandler_ListItems_Success(t *testing.T) {
	m := newMocks()
	next := 60
	m.listItems.out = []domainexpiry.ItemView{{
		Item: domainexpiry.Item{
			ID: uuid.New(), Type: domainexpiry.TypeTLSCertificate,
			Subject: "example.com:443", ExpiresAt: time.Now().Add(62 * 24 * time.Hour),
			ObservedAt: time.Now(), Meta: map[string]string{"issuer": "Let's Encrypt"},
		},
		DaysLeft: 62, State: domainexpiry.StateOK, NextThresholdDays: &next, Issuer: "Let's Encrypt",
	}}
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/expiry-items?type=tls_certificate", nil)
	assertStatus(t, resp, http.StatusOK)
	var out presenter.ExpiryItemsResponse
	testutil.DecodeJSON(t, resp, &out)
	if len(out.Items) != 1 || out.Items[0].State != "ok" || out.Items[0].Issuer == "" {
		t.Fatalf("unexpected %+v", out)
	}
}

func TestExpiryHandler_ListPolicies_NotFound(t *testing.T) {
	m := newMocks()
	m.listPolicies.err = domainworkflow.ErrNotFound
	resp := doJSON(t, m, true, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/expiry-policies", nil)
	assertStatus(t, resp, http.StatusNotFound)
}

func TestExpiryHandler_Unauthorized(t *testing.T) {
	m := newMocks()
	resp := doJSON(t, m, false, http.MethodGet, "/workflows/"+testWorkflowID.String()+"/expiry-policies", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}
