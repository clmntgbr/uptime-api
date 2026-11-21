package mail

import "go-api/internal/contract"

func init() { register(expiryExpired{}) }

type expiryExpired struct{}

func (expiryExpired) Kind() string { return "expiry.expired" }

func (expiryExpired) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Certificate expired",
		Subject: certificateSubject(req),
		Message: req.Message,
		Rows:    expiryRows(req, false),
	}
}
