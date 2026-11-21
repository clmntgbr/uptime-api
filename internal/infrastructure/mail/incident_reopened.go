package mail

import "go-api/internal/contract"

func init() { register(incidentReopened{}) }

type incidentReopened struct{}

func (incidentReopened) Kind() string { return "incident.reopened" }

func (incidentReopened) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Incident reopened",
		Subject: incidentSubject(req, "reopened"),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
