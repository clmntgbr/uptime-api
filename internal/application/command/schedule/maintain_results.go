package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
)

type MaintainResultsHandler struct {
	retention domainrun.Retention
	clock     port.Clock
	keepFor   time.Duration
	logger    *slog.Logger
}

func NewMaintainResultsHandler(
	retention domainrun.Retention,
	clock port.Clock,
	keepFor time.Duration,
	logger *slog.Logger,
) *MaintainResultsHandler {
	return &MaintainResultsHandler{retention: retention, clock: clock, keepFor: keepFor, logger: logger}
}

// PurgeExpiredRuns deletes the runs older than the retention.
func (h *MaintainResultsHandler) PurgeExpiredRuns(ctx context.Context) error {
	deleted, err := h.retention.DeleteRunsBefore(ctx, h.clock.Now().Add(-h.keepFor))
	if err != nil {
		return fmt.Errorf("delete expired runs: %w", err)
	}
	if deleted > 0 {
		h.logger.Info("expired runs deleted", "count", deleted)
	}
	return nil
}
