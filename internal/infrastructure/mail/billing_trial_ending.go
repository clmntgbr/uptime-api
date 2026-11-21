package mail

import "go-api/internal/contract"

func init() { register(billingTrialEnding{}) }

type billingTrialEnding struct{}

func (billingTrialEnding) Kind() string { return "billing.trial_ending" }

func (billingTrialEnding) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Billing trial ending",
		Subject: "[Uptime] Billing · trial_ending",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
