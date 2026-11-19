package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
)

// EventRetention purges transactional event tables past the retention window.
type EventRetention interface {
	DeletePublishedOutboxBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type MaintainResultsHandler struct {
	retention domainrun.Retention
	events    EventRetention
	clock     port.Clock
	keepFor   time.Duration
	logger    *slog.Logger
}

// WithEventRetention enables purging of the outbox and dedup tables.
func (h *MaintainResultsHandler) WithEventRetention(events EventRetention) *MaintainResultsHandler {
	h.events = events
	return h
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
	if h.events == nil {
		return nil
	}
	cutoff := h.clock.Now().Add(-h.keepFor)
	outboxDeleted, err := h.events.DeletePublishedOutboxBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("delete published outbox: %w", err)
	}
	processedDeleted, err := h.events.DeleteProcessedBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("delete processed events: %w", err)
	}
	if outboxDeleted > 0 || processedDeleted > 0 {
		h.logger.Info("expired events deleted", "outbox", outboxDeleted, "processed", processedDeleted)
	}
	return nil
}
