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

func TestRenderWorkflowDisabled(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "workflow.disabled", Subject: "Checkout", Severity: "warning",
		WorkflowID: "wf", Message: `Workflow "Checkout" was disabled because it has no calls.`,
	})
	if !strings.Contains(subj, "Workflow disabled") || !strings.Contains(subj, "Checkout") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "no calls") || !strings.Contains(body, "workflow.disabled") {
		t.Fatalf("body: %s", body)
	}
}

func TestRenderQuota(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "quota.warning", Severity: "warning", WorkflowID: "wf",
		Message: "You reached 80% of your run quota.",
	})
	if !strings.Contains(subj, "Quota") || !strings.Contains(subj, "warning") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "80%") || !strings.Contains(body, "quota.warning") {
		t.Fatalf("body: %s", body)
	}
}

func TestRenderRunAndOneShot(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "run.failed", Type: "failed", Severity: "critical",
		WorkflowID: "wf", ItemID: "run-1", Message: "Workflow run finished with status failed",
	})
	if !strings.Contains(subj, "Run") || !strings.Contains(subj, "failed") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "run.failed") {
		t.Fatalf("body: %s", body)
	}
	subj, body = Render(contract.NotificationRequest{
		Kind: "workflow.one_shot.done", Subject: "Nightly", Severity: "info",
		WorkflowID: "wf", Message: `One-shot workflow "Nightly" finished and was disabled.`,
	})
	if !strings.Contains(subj, "One-shot") || !strings.Contains(subj, "Nightly") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "disabled") {
		t.Fatalf("body: %s", body)
	}
}

func TestRenderExpiryExpired(t *testing.T) {
	subj, body := Render(contract.NotificationRequest{
		Kind: "expiry.expired", Subject: "api.example.com:443",
		DaysLeft: -1, Severity: "critical", WorkflowID: "wf",
		Message: "The certificate of api.example.com:443 has expired",
	})
	if !strings.Contains(subj, "Certificate") {
		t.Fatalf("subject: %s", subj)
	}
	if !strings.Contains(body, "has expired") || strings.Contains(body, "Threshold") {
		t.Fatalf("body: %s", body)
	}
}
