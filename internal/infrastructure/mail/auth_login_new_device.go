package mail

import "go-api/internal/contract"

func init() { register(authLoginNewDevice{}) }

type authLoginNewDevice struct{}

func (authLoginNewDevice) Kind() string { return "auth.login_new_device" }

func (authLoginNewDevice) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Account login new device",
		Subject: "[Uptime] Account · login_new_device",
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
