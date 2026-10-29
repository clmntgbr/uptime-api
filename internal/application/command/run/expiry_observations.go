package run

import (
	"time"

	"go-api/internal/contract"
	domainexpiry "go-api/internal/domain/expiry"
)

func observationsFromResult(result contract.Result) []domainexpiry.Observation {
	seen := map[string]struct{}{}
	var out []domainexpiry.Observation
	for _, c := range result.Calls {
		if c.TLS == nil || c.TLS.Host == "" || c.TLS.ExpiresAt.IsZero() {
			continue
		}
		fp := c.TLS.FingerprintSHA256
		if fp == "" {
			fp = c.TLS.SerialNumber
		}
		if fp == "" {
			continue
		}
		subject := domainexpiry.SubjectHostPort(c.TLS.Host, c.TLS.Port)
		if _, dup := seen[subject]; dup {
			continue
		}
		seen[subject] = struct{}{}
		observedAt := c.FinishedAt
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		out = append(out, domainexpiry.Observation{
			Type:        domainexpiry.TypeTLSCertificate,
			Subject:     subject,
			ExpiresAt:   c.TLS.ExpiresAt.UTC(),
			Fingerprint: fp,
			Meta: map[string]string{
				"issuer":  c.TLS.Issuer,
				"subject": c.TLS.Subject,
				"host":    c.TLS.Host,
			},
			ObservedAt: observedAt,
		})
	}
	return out
}

func toContractNotifications(notes []domainexpiry.NotificationRequest) []contract.NotificationRequest {
	out := make([]contract.NotificationRequest, 0, len(notes))
	for _, n := range notes {
		out = append(out, contract.NotificationRequest{
			V: contract.NotificationRequestV1, Kind: n.Kind, Type: string(n.Type),
			UserID: n.UserID.String(), WorkflowID: n.WorkflowID.String(),
			ItemID: n.ItemID.String(), Subject: n.Subject,
			DaysLeft: n.DaysLeft, ThresholdDays: n.ThresholdDays,
			ExpiresAt: n.ExpiresAt, Severity: string(n.Severity), Message: n.Message,
		})
	}
	return out
}
