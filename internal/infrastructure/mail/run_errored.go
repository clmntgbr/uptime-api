package mail

import (
	"fmt"

	"go-api/internal/contract"
)

func init() { register(runErrored{}) }

type runErrored struct{}

func (runErrored) Kind() string { return "run.errored" }

func (runErrored) Build(req contract.NotificationRequest) Content {
	status := fallback(req.Type, "error")
	return Content{
		Title:   "Run " + status,
		Subject: fmt.Sprintf("[Uptime] Run · %s", status),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
