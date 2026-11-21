package mail

import (
	"fmt"

	"go-api/internal/contract"
)

func init() { register(runSuccess{}) }

type runSuccess struct{}

func (runSuccess) Kind() string { return "run.success" }

func (runSuccess) Build(req contract.NotificationRequest) Content {
	status := fallback(req.Type, "success")
	return Content{
		Title:   "Run " + status,
		Subject: fmt.Sprintf("[Uptime] Run · %s", status),
		Message: req.Message,
		Rows:    baseRows(req),
	}
}
