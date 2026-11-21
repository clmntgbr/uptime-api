package mail

import "go-api/internal/contract"

func init() { register(billingPaymentFailed{}) }

type billingPaymentFailed struct{}

func (billingPaymentFailed) Kind() string { return "billing.payment_failed" }

func (billingPaymentFailed) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Billing payment failed",
		Subject: "[Uptime] Billing · payment_failed",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
