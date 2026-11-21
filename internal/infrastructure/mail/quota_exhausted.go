package mail

import "go-api/internal/contract"

func init() { register(quotaExhausted{}) }

type quotaExhausted struct{}

func (quotaExhausted) Kind() string { return "quota.exhausted" }

func (quotaExhausted) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Quota exhausted",
		Subject: "[Uptime] Quota · exhausted",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
