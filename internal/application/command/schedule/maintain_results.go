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
	retention  domainrun.Retention
	aggregates domainrun.AggregateRepository
	clock      port.Clock
	keepFor    time.Duration
	logger     *slog.Logger
}

func NewMaintainResultsHandler(
	retention domainrun.Retention,
	aggregates domainrun.AggregateRepository,
	clock port.Clock,
	keepFor time.Duration,
	logger *slog.Logger,
) *MaintainResultsHandler {
	return &MaintainResultsHandler{retention: retention, aggregates: aggregates, clock: clock, keepFor: keepFor, logger: logger}
}

// ComputeAggregates refreshes the current and previous buckets: late results
// (redeliveries, timeouts) can still land in the previous hour or day.
func (h *MaintainResultsHandler) ComputeAggregates(ctx context.Context) error {
	now := h.clock.Now().UTC()
	hour := now.Truncate(time.Hour)
	if _, err := h.aggregates.ComputeHourly(ctx, hour.Add(-time.Hour), hour.Add(time.Hour)); err != nil {
		return fmt.Errorf("hourly aggregates: %w", err)
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if _, err := h.aggregates.ComputeDaily(ctx, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1)); err != nil {
		return fmt.Errorf("daily aggregates: %w", err)
	}
	return nil
}

// PurgeExpiredRuns deletes the runs older than the retention. Aggregates are kept.
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
