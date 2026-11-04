package mail

import (
	"fmt"
	"html"
	"strings"
	"time"

	"go-api/internal/contract"
)

// Render builds subject + HTML body for a notification request.
func Render(req contract.NotificationRequest) (subject, htmlBody string) {
	switch {
	case strings.HasPrefix(req.Kind, "expiry."):
		return renderTLS(req)
	case strings.HasPrefix(req.Kind, "incident."):
		return renderIncident(req)
	default:
		return renderBase(req.Kind, req.Severity, req.Message, nil)
	}
}

func renderTLS(req contract.NotificationRequest) (string, string) {
	subject := fmt.Sprintf("[Uptime] Certificate · %s", fallback(req.Subject, "TLS"))
	rows := []kv{
		{"Alert", req.Kind},
		{"Host", fallback(req.Subject, "—")},
		{"Days left", fmt.Sprintf("%d", req.DaysLeft)},
		{"Threshold", fmt.Sprintf("%d days", req.ThresholdDays)},
		{"Severity", fallback(req.Severity, "—")},
	}
	if !req.ExpiresAt.IsZero() {
		rows = append(rows, kv{"Expires at", req.ExpiresAt.UTC().Format(time.RFC3339)})
	}
	rows = append(rows, kv{"Workflow", req.WorkflowID})
	return subject, wrap("Certificate expiry", req.Message, rows)
}

func renderIncident(req contract.NotificationRequest) (string, string) {
	label := strings.TrimPrefix(req.Kind, "incident.")
	title := "Incident " + label
	subject := fmt.Sprintf("[Uptime] Incident · %s", fallback(req.Type, label))
	rows := []kv{
		{"Event", req.Kind},
		{"Kind", fallback(req.Type, "—")},
		{"Severity", fallback(req.Severity, "—")},
		{"Incident", fallback(req.IncidentID, req.ItemID, "—")},
		{"Workflow", req.WorkflowID},
	}
	return subject, wrap(title, req.Message, rows)
}

func renderBase(kind, severity, message string, extra []kv) (string, string) {
	subject := fmt.Sprintf("[Uptime] %s", fallback(kind, "Notification"))
	rows := []kv{
		{"Kind", fallback(kind, "—")},
		{"Severity", fallback(severity, "—")},
	}
	rows = append(rows, extra...)
	return subject, wrap("Notification", message, rows)
}

type kv struct{ k, v string }

func wrap(title, message string, rows []kv) string {
	var details strings.Builder
	for _, row := range rows {
		if strings.TrimSpace(row.v) == "" {
			continue
		}
		details.WriteString(fmt.Sprintf(
			`<tr><td style="padding:6px 12px 6px 0;color:#64748b;vertical-align:top;">%s</td><td style="padding:6px 0;color:#0f172a;">%s</td></tr>`,
			html.EscapeString(row.k), html.EscapeString(row.v),
		))
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#f8fafc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;padding:24px 12px;">
    <tr><td align="center">
      <table role="presentation" width="560" cellspacing="0" cellpadding="0" style="background:#ffffff;border:1px solid #e2e8f0;border-radius:8px;padding:24px;">
        <tr><td style="font-size:18px;font-weight:600;color:#0f172a;padding-bottom:8px;">%s</td></tr>
        <tr><td style="font-size:14px;line-height:1.5;color:#334155;padding-bottom:16px;">%s</td></tr>
        <tr><td>
          <table role="presentation" cellspacing="0" cellpadding="0" style="font-size:13px;width:100%%;">%s</table>
        </td></tr>
        <tr><td style="padding-top:20px;font-size:12px;color:#94a3b8;">Uptime · notification</td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(message),
		details.String(),
	)
}

func fallback(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
