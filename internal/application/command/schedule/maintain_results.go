package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domainfeed "go-api/internal/domain/feed"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
)

// EventRetention purges transactional event tables past the retention window.
type EventRetention interface {
	DeletePublishedOutboxBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type MaintainResultsHandler struct {
	retention   domainrun.Retention
	events      EventRetention
	feed        domainfeed.WriteRepository
	clock       port.Clock
	keepFor     time.Duration
	feedKeepFor time.Duration
	logger      *slog.Logger
}

// WithEventRetention enables purging of the outbox and dedup tables.
func (h *MaintainResultsHandler) WithEventRetention(events EventRetention) *MaintainResultsHandler {
	h.events = events
	return h
}

// WithFeedRetention enables purging of feed_events past feedKeepFor.
func (h *MaintainResultsHandler) WithFeedRetention(repo domainfeed.WriteRepository, keepFor time.Duration) *MaintainResultsHandler {
	h.feed = repo
	h.feedKeepFor = keepFor
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
	if h.feed != nil && h.feedKeepFor > 0 {
		feedDeleted, err := h.feed.DeleteBefore(ctx, h.clock.Now().Add(-h.feedKeepFor))
		if err != nil {
			return fmt.Errorf("delete expired feed: %w", err)
		}
		if feedDeleted > 0 {
			h.logger.Info("expired feed events deleted", "count", feedDeleted)
		}
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
