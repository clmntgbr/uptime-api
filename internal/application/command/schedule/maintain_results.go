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
	partitions domainrun.PartitionMaintainer
	aggregates domainrun.AggregateRepository
	clock      port.Clock
	retention  time.Duration
	logger     *slog.Logger
}

func NewMaintainResultsHandler(
	partitions domainrun.PartitionMaintainer,
	aggregates domainrun.AggregateRepository,
	clock port.Clock,
	retention time.Duration,
	logger *slog.Logger,
) *MaintainResultsHandler {
	return &MaintainResultsHandler{partitions: partitions, aggregates: aggregates, clock: clock, retention: retention, logger: logger}
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

// RotatePartitions creates the next monthly partitions and drops the ones
// entirely older than the retention. Aggregates are kept.
func (h *MaintainResultsHandler) RotatePartitions(ctx context.Context) error {
	if err := h.partitions.EnsurePartitions(ctx, 2); err != nil {
		return fmt.Errorf("ensure partitions: %w", err)
	}
	dropped, err := h.partitions.DropPartitionsBefore(ctx, h.clock.Now().Add(-h.retention))
	if err != nil {
		return fmt.Errorf("drop partitions: %w", err)
	}
	for _, name := range dropped {
		h.logger.Info("result partition dropped", "partition", name)
	}
	return nil
}
