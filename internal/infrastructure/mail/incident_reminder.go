package mail

import "go-api/internal/contract"

func init() { register(incidentReminder{}) }

type incidentReminder struct{}

func (incidentReminder) Kind() string { return "incident.reminder" }

func (incidentReminder) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Incident reminder",
		Subject: incidentSubject(req, "reminder"),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
