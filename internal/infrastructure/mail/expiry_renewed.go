package mail

import "go-api/internal/contract"

func init() { register(expiryRenewed{}) }

type expiryRenewed struct{}

func (expiryRenewed) Kind() string { return "expiry.renewed" }

func (expiryRenewed) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Certificate renewed",
		Subject: certificateSubject(req),
		Message: req.Message,
		Rows:    expiryRows(req, false),
	}
}
