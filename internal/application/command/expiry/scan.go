package expiry

import (
	"context"
	"log/slog"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
)

type ScanHandler struct {
	items    domainexpiry.ItemRepository
	evaluate *EvaluateHandler
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
		note, err := h.evaluate.Evaluate(ctx, item)
		if err != nil {
			h.logger.Error("expiry scan evaluate failed", "item_id", item.ID, "error", err)
			continue
		}
		if note != nil {
			notes = append(notes, *note)
		}
	}
	return notes, nil
}
