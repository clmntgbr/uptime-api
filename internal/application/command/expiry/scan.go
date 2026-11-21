package expiry

import (
	"context"
	"log/slog"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
	domainfeed "go-api/internal/domain/feed"
)

type ScanHandler struct {
	items    domainexpiry.ItemRepository
	evaluate *EvaluateHandler
	feedRepo domainfeed.WriteRepository
	logger   *slog.Logger
	clock    func() time.Time
}

func NewScanHandler(
	items domainexpiry.ItemRepository,
	evaluate *EvaluateHandler,
	logger *slog.Logger,
) *ScanHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanHandler{items: items, evaluate: evaluate, logger: logger, clock: time.Now}
}

// WithFeed enables append of tls/expiry threshold feed rows on claim.
func (h *ScanHandler) WithFeed(repo domainfeed.WriteRepository) *ScanHandler {
	h.feedRepo = repo
	return h
}

// Handle evaluates the expiry items that can still raise a notification (daily
// scheduler pass). Items outside domainexpiry.ScanWindow are skipped in SQL.
func (h *ScanHandler) Handle(ctx context.Context) ([]domainexpiry.NotificationRequest, error) {
	from, to := domainexpiry.ScanWindow(h.clock().UTC())
	items, err := h.items.ListExpiringBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var notes []domainexpiry.NotificationRequest
	for _, item := range items {
		outcome, err := h.evaluate.Evaluate(ctx, item)
		if err != nil {
			h.logger.Error("expiry scan evaluate failed", "item_id", item.ID, "error", err)
			continue
		}
		if outcome.Notification != nil {
			notes = append(notes, *outcome.Notification)
		}
		if h.feedRepo != nil {
			if evs := feedEventsFromOutcome(outcome); len(evs) > 0 {
				if err := h.feedRepo.Append(ctx, evs); err != nil {
					return notes, err
				}
			}
		}
	}
	return notes, nil
}
