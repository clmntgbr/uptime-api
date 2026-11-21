package mail

import (
	"fmt"
	"html"
	"strings"
)

// Content is what every email fills before the shared HTML layout.
type Content struct {
	Title   string
	Subject string
	Message string
	Rows    []kv
}

type kv struct{ k, v string }

// renderLayout wraps title, message and detail rows in the general HTML shell.
func renderLayout(c Content) (subject, htmlBody string) {
	var details strings.Builder
	for _, row := range c.Rows {
		if strings.TrimSpace(row.v) == "" {
			continue
		}
		details.WriteString(fmt.Sprintf(
			`<tr><td style="padding:6px 12px 6px 0;color:#64748b;vertical-align:top;">%s</td><td style="padding:6px 0;color:#0f172a;">%s</td></tr>`,
			html.EscapeString(row.k), html.EscapeString(row.v),
		))
	}
	body := fmt.Sprintf(`<!DOCTYPE html>
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
		html.EscapeString(c.Title),
		html.EscapeString(c.Message),
		details.String(),
	)
	return c.Subject, body
}

func fallback(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func humanizeKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return ""
	}
	return strings.ReplaceAll(kind, ".", " · ")
}

func row(k, v string) kv { return kv{k: k, v: v} }
