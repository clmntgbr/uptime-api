package mail

import "go-api/internal/contract"

func init() { register(expiryThreshold{}) }

type expiryThreshold struct{}

func (expiryThreshold) Kind() string { return "expiry.threshold" }

func (expiryThreshold) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Certificate expiry",
		Subject: certificateSubject(req),
		Message: req.Message,
		Rows:    expiryRows(req, true),
	}
}
