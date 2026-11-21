package mail

import (
	"fmt"

	"go-api/internal/contract"
)

func init() { register(runFailed{}) }

type runFailed struct{}

func (runFailed) Kind() string { return "run.failed" }

func (runFailed) Build(req contract.NotificationRequest) Content {
	status := fallback(req.Type, "failed")
	return Content{
		Title:   "Run " + status,
		Subject: fmt.Sprintf("[Uptime] Run · %s", status),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
