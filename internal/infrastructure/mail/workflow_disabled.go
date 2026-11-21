package mail

import (
	"fmt"

	"go-api/internal/contract"
)

func init() { register(workflowDisabled{}) }

type workflowDisabled struct{}

func (workflowDisabled) Kind() string { return "workflow.disabled" }

func (workflowDisabled) Build(req contract.NotificationRequest) Content {
	name := fallback(req.Subject, "Workflow")
	return Content{
		Title:   "Workflow disabled",
		Subject: fmt.Sprintf("[Uptime] Workflow disabled · %s", name),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
