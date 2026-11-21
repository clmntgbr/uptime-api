package mail

import "go-api/internal/contract"

func init() { register(quotaWarning{}) }

type quotaWarning struct{}

func (quotaWarning) Kind() string { return "quota.warning" }

func (quotaWarning) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Quota warning",
		Subject: "[Uptime] Quota · warning",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
