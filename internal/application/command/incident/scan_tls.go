package incident

import (
	"context"
	"log/slog"
	"time"

	cmdexpiry "go-api/internal/application/command/expiry"
	domainexpiry "go-api/internal/domain/expiry"
	domainincident "go-api/internal/domain/incident"
)

// ScanTLSHandler feeds tls_expiry signals from every known expiry item (daily pass).
type ScanTLSHandler struct {
	tx             txRunner
	items          domainexpiry.ItemRepository
	expiryPolicies domainexpiry.PolicyRepository
	apply          *ApplySignalsHandler
	logger         *slog.Logger
	clock          func() time.Time
}

func NewScanTLSHandler(
	tx txRunner,
	items domainexpiry.ItemRepository,
	expiryPolicies domainexpiry.PolicyRepository,
	apply *ApplySignalsHandler,
	logger *slog.Logger,
) *ScanTLSHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanTLSHandler{
		tx: tx, items: items, expiryPolicies: expiryPolicies,
		apply: apply, logger: logger, clock: time.Now,
	}
}

func (h *ScanTLSHandler) Handle(ctx context.Context) (ApplyResult, error) {
	var result ApplyResult
	now := h.clock().UTC()
	items, err := h.items.ListAllForScan(ctx)
	if err != nil {
		return result, err
	}
	for _, item := range items {
		policy, err := cmdexpiry.ResolvePolicy(ctx, h.expiryPolicies, item.UserID, item.WorkflowID, item.Type)
		if err != nil {
			h.logger.Error("tls scan policy resolve failed", "item_id", item.ID, "error", err)
			continue
		}
		daysLeft := domainexpiry.DaysLeft(item.ExpiresAt, now)
		sig := domainincident.TLSSignal(item.Subject, daysLeft, policy.ThresholdsDays, now)
		if sig == nil {
			continue
		}
		var applied ApplyResult
		err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			var applyErr error
			applied, applyErr = h.apply.Handle(txCtx, ApplySignalsCommand{
				UserID: item.UserID, WorkflowID: item.WorkflowID,
				Signals: []domainincident.Signal{*sig},
			})
			return applyErr
		})
		if err != nil {
			h.logger.Error("tls incident apply failed", "item_id", item.ID, "error", err)
			continue
		}
		result.Notifications = append(result.Notifications, applied.Notifications...)
		result.DomainEvents = append(result.DomainEvents, applied.DomainEvents...)
	}
	return result, nil
}
