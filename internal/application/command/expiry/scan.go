package expiry

import (
	"context"
	"log/slog"

	domainexpiry "go-api/internal/domain/expiry"
)

type ScanHandler struct {
	items    domainexpiry.ItemRepository
	evaluate *EvaluateHandler
	logger   *slog.Logger
}

func NewScanHandler(
	items domainexpiry.ItemRepository,
	evaluate *EvaluateHandler,
	logger *slog.Logger,
) *ScanHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanHandler{items: items, evaluate: evaluate, logger: logger}
}

// Handle evaluates every known expiry item (daily scheduler pass).
func (h *ScanHandler) Handle(ctx context.Context) ([]domainexpiry.NotificationRequest, error) {
	items, err := h.items.ListAllForScan(ctx)
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
