package mail

import "go-api/internal/contract"

func init() { register(authEmailChanged{}) }

type authEmailChanged struct{}

func (authEmailChanged) Kind() string { return "auth.email_changed" }

func (authEmailChanged) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Account email changed",
		Subject: "[Uptime] Account · email_changed",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
