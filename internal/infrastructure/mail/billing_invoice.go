package mail

import "go-api/internal/contract"

func init() { register(billingInvoice{}) }

type billingInvoice struct{}

func (billingInvoice) Kind() string { return "billing.invoice" }

func (billingInvoice) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Billing invoice",
		Subject: "[Uptime] Billing · invoice",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
