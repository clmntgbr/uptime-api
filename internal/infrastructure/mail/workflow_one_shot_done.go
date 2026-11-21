package mail

import (
	"fmt"

	"go-api/internal/contract"
)

func init() { register(workflowOneShotDone{}) }

type workflowOneShotDone struct{}

func (workflowOneShotDone) Kind() string { return "workflow.one_shot.done" }

func (workflowOneShotDone) Build(req contract.NotificationRequest) Content {
	name := fallback(req.Subject, "Workflow")
	return Content{
		Title:   "One-shot finished",
		Subject: fmt.Sprintf("[Uptime] One-shot done · %s", name),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
