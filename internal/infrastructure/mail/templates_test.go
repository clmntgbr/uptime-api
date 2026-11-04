package mail

import (
	"strings"
	"testing"
	"time"

	"go-api/internal/contract"
)

func TestRenderTLS(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "expiry.threshold", Subject: "example.com:443",
		DaysLeft: 7, ThresholdDays: 7, Severity: "warning",
		WorkflowID: "wf", Message: "The certificate of example.com:443 expires in 7 days",
		ExpiresAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
	})
	if !strings.Contains(subj, "Certificate") || !strings.Contains(subj, "example.com:443") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "Days left") || !strings.Contains(body, "expires in 7 days") {
		t.Fatalf("body: %s", body)
	}
}

func TestRenderIncident(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "incident.reminder", Type: "call_failed", Severity: "critical",
		IncidentID: "inc-1", WorkflowID: "wf", Message: "Incident call_failed still open",
	})
	if !strings.Contains(subj, "Incident") || !strings.Contains(subj, "call_failed") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "incident.reminder") || !strings.Contains(body, "still open") {
		t.Fatalf("body: %s", body)
	}
}
