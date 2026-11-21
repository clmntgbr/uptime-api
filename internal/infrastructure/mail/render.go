package mail

import (
	"fmt"
	"time"

	"go-api/internal/contract"
)

// email is one notification kind: subject/title/details, then the shared layout.
type email interface {
	Kind() string
	Build(req contract.NotificationRequest) Content
}

var registry = map[string]email{}

func register(e email) {
	if e == nil || e.Kind() == "" {
		panic("mail: register requires a non-empty kind")
	}
	if _, exists := registry[e.Kind()]; exists {
		panic("mail: duplicate email kind " + e.Kind())
	}
	registry[e.Kind()] = e
}

// Render builds subject + HTML body from a notification request.
func Render(req contract.NotificationRequest) (subject, htmlBody string) {
	e, ok := registry[req.Kind]
	if !ok {
		e = fallbackEmail{}
	}
	return renderLayout(e.Build(req))
}

func baseRows(req contract.NotificationRequest, extra ...kv) []kv {
	rows := []kv{
		row("Event", req.Kind),
		row("Type", req.Type),
		row("Severity", req.Severity),
		row("Subject", req.Subject),
	}
	rows = append(rows, extra...)
	rows = append(rows,
		row("Incident", fallback(req.IncidentID, "")),
		row("Item", req.ItemID),
		row("Workflow", req.WorkflowID),
	)
	return rows
}

func expiryRows(req contract.NotificationRequest, withThreshold bool) []kv {
	extra := []kv{row("Days left", fmt.Sprintf("%d", req.DaysLeft))}
	if withThreshold {
		extra = append(extra, row("Threshold", fmt.Sprintf("%d days", req.ThresholdDays)))
	}
	if !req.ExpiresAt.IsZero() {
		extra = append(extra, row("Expires at", req.ExpiresAt.UTC().Format(time.RFC3339)))
	}
	return baseRows(req, extra...)
}

func incidentSubject(req contract.NotificationRequest, action string) string {
	kind := fallback(req.Type, action)
	return fmt.Sprintf("[Uptime] Incident · %s", kind)
}

func certificateSubject(req contract.NotificationRequest) string {
	return fmt.Sprintf("[Uptime] Certificate · %s", fallback(req.Subject, "TLS"))
}

type fallbackEmail struct{}

func (fallbackEmail) Kind() string { return "" }

func (fallbackEmail) Build(req contract.NotificationRequest) Content {
	label := fallback(humanizeKind(req.Kind), "Notification")
	return Content{
		Title:   label,
		Subject: fmt.Sprintf("[Uptime] %s", label),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
