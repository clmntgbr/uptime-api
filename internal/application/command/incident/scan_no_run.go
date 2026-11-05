package incident

import (
	"context"
	"log/slog"
	"time"

	domainincident "go-api/internal/domain/incident"
)

type ScanNoRunHandler struct {
	tx     txRunner
	scan   domainincident.ScanRepository
	apply  *ApplySignalsHandler
	logger *slog.Logger
	clock  func() time.Time
}

func NewScanNoRunHandler(
	tx txRunner,
	scan domainincident.ScanRepository,
	apply *ApplySignalsHandler,
	logger *slog.Logger,
) *ScanNoRunHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScanNoRunHandler{tx: tx, scan: scan, apply: apply, logger: logger, clock: time.Now}
}

// Handle emits no_run failing/healthy signals for enabled interval workflows.
// schedulerLag skips workflows whose interval is shorter than the lag (platform late).
func (h *ScanNoRunHandler) Handle(ctx context.Context, schedulerLag time.Duration) (ApplyResult, error) {
	var result ApplyResult
	now := h.clock().UTC()
	workflows, err := h.scan.ListEnabledIntervalWorkflows(ctx)
	if err != nil {
		return result, err
	}
	if len(workflows) == 0 {
		return result, nil
	}

	type candidate struct {
		wf     domainincident.EnabledWorkflow
		signal domainincident.Signal
	}
	var candidates []candidate
	silent := 0
	for _, wf := range workflows {
		if wf.Interval <= 0 {
			continue
		}
		if schedulerLag > wf.Interval {
			continue
		}
		policy, err := h.apply.resolvePolicy(ctx, wf.UserID, wf.WorkflowID, domainincident.KindNoRun)
		if err != nil {
			return result, err
		}
		if !policy.Enabled {
			continue
		}
		deadline := time.Duration(policy.NoRunMultiplier()) * wf.Interval
		anchor := wf.SilenceAnchor()
		isSilent := now.Sub(anchor) > deadline
		sig := domainincident.Signal{
			Kind: domainincident.KindNoRun, Failing: isSilent, ObservedAt: now,
			Cause: map[string]any{
				"silenceSince": anchor,
				"deadline":     deadline.String(),
				"multiplier":   policy.NoRunMultiplier(),
			},
		}
		if isSilent {
			silent++
		}
		candidates = append(candidates, candidate{wf: wf, signal: sig})
	}

	suppressFailing := domainincident.ShouldSuppressNoRun(
		silent, len(candidates),
		domainincident.NoRunSuppressMinCount, domainincident.NoRunSuppressMinRatio,
	)
	if suppressFailing {
		h.logger.Warn("no_run suppressed (mass silence)",
			"silent", silent, "enabled", len(candidates))
	}

	for _, c := range candidates {
		if suppressFailing && c.signal.Failing {
			continue
		}
		var applied ApplyResult
		err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			var applyErr error
			applied, applyErr = h.apply.Handle(txCtx, ApplySignalsCommand{
				UserID: c.wf.UserID, WorkflowID: c.wf.WorkflowID,
				Signals: []domainincident.Signal{c.signal},
			})
			return applyErr
		})
		if err != nil {
			h.logger.Error("no_run apply failed", "workflow_id", c.wf.WorkflowID, "error", err)
			continue
		}
		result.Notifications = append(result.Notifications, applied.Notifications...)
		result.DomainEvents = append(result.DomainEvents, applied.DomainEvents...)
	}
	return result, nil
}
