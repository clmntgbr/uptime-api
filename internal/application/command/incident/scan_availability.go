package incident

import (
	"context"
	"log/slog"
	"time"

	domainincident "go-api/internal/domain/incident"
)

type ScanAvailabilityHandler struct {
	tx     txRunner
	scan   domainincident.ScanRepository
	apply  *ApplySignalsHandler
	logger *slog.Logger
	clock  func() time.Time
}

func NewScanAvailabilityHandler(
	tx txRunner,
	scan domainincident.ScanRepository,
	apply *ApplySignalsHandler,
	logger *slog.Logger,
) *ScanAvailabilityHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanAvailabilityHandler{tx: tx, scan: scan, apply: apply, logger: logger, clock: time.Now}
}

// Handle evaluates availability_low for workflows with an enabled stored policy.
func (h *ScanAvailabilityHandler) Handle(ctx context.Context) (ApplyResult, error) {
	var result ApplyResult
	now := h.clock().UTC()
	workflows, err := h.scan.ListAvailabilityEnabledWorkflows(ctx)
	if err != nil {
		return result, err
	}
	for _, wf := range workflows {
		policy, err := h.apply.resolvePolicy(ctx, wf.UserID, wf.WorkflowID, domainincident.KindAvailabilityLow)
		if err != nil {
			return result, err
		}
		if !policy.Enabled {
			continue
		}
		ratio, total, err := h.scan.RunSuccessRatio24h(ctx, wf.WorkflowID, now)
		if err != nil {
			h.logger.Error("availability ratio failed", "workflow_id", wf.WorkflowID, "error", err)
			continue
		}
		if total == 0 {
			continue
		}
		th, rec := policy.AvailabilityThresholds()
		sig := domainincident.AvailabilitySignal(ratio, th, rec, now)
		if sig == nil {
			continue
		}
		var applied ApplyResult
		err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			var applyErr error
			applied, applyErr = h.apply.Handle(txCtx, ApplySignalsCommand{
				UserID: wf.UserID, WorkflowID: wf.WorkflowID,
				Signals: []domainincident.Signal{*sig},
			})
			return applyErr
		})
		if err != nil {
			h.logger.Error("availability apply failed", "workflow_id", wf.WorkflowID, "error", err)
			continue
		}
		result.Notifications = append(result.Notifications, applied.Notifications...)
		result.DomainEvents = append(result.DomainEvents, applied.DomainEvents...)
	}
	return result, nil
}
