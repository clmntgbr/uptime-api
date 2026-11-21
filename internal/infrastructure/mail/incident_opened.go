package mail

import "go-api/internal/contract"

func init() { register(incidentOpened{}) }

type incidentOpened struct{}

func (incidentOpened) Kind() string { return "incident.opened" }

func (incidentOpened) Build(req contract.NotificationRequest) Content {
	return Content{
		Title:   "Incident opened",
		Subject: incidentSubject(req, "opened"),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
