package mail

import "go-api/internal/contract"

func init() { register(incidentResolved{}) }

type incidentResolved struct{}

func (incidentResolved) Kind() string { return "incident.resolved" }

func (incidentResolved) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Incident resolved",
		Subject: incidentSubject(req, "resolved"),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
