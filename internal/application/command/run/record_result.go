package run

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	cmdexpiry "go-api/internal/application/command/expiry"
	cmdincident "go-api/internal/application/command/incident"
	"go-api/internal/application/command/schedule"
	"go-api/internal/application/messaging"
	"go-api/internal/application/notifyresolve"
	"go-api/internal/contract"
	"go-api/internal/domain/certificate"
	domainexpiry "go-api/internal/domain/expiry"
	domainfeed "go-api/internal/domain/feed"
	domainincident "go-api/internal/domain/incident"
	domainnotify "go-api/internal/domain/notifysettings"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type notificationPublisher interface {
	PublishRequest(ctx context.Context, req contract.NotificationRequest) error
}

type RecordResultHandler struct {
	states         domainschedule.Repository
	workflows      domainworkflow.WriteRepository
	runs           domainrun.WriteRepository
	certificates   certificate.Repository
	expiry         *cmdexpiry.RecordObservationsHandler
	expiryPolicies domainexpiry.PolicyRepository
	incidents      *cmdincident.ApplySignalsHandler
	notifySettings domainnotify.Repository
	nestedTx       port.NestedTransaction
	notify         notificationPublisher
	processed      port.ProcessedEventRepository
	outbox         port.OutboxRepository
	feedRepo       domainfeed.WriteRepository
	logger         *slog.Logger
	clock          port.Clock
}

func NewRecordResultHandler(
	states domainschedule.Repository,
	workflows domainworkflow.WriteRepository,
	runs domainrun.WriteRepository,
	certificates certificate.Repository,
	expiry *cmdexpiry.RecordObservationsHandler,
	expiryPolicies domainexpiry.PolicyRepository,
	incidents *cmdincident.ApplySignalsHandler,
	notifySettings domainnotify.Repository,
	nestedTx port.NestedTransaction,
	notify notificationPublisher,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
	logger *slog.Logger,
	clock port.Clock,
) *RecordResultHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecordResultHandler{
		states: states, workflows: workflows, runs: runs,
		certificates: certificates, expiry: expiry, expiryPolicies: expiryPolicies,
		incidents: incidents, notifySettings: notifySettings,
		nestedTx: nestedTx, notify: notify,
		processed: processed, outbox: outbox, logger: logger, clock: clock,
	}
}

// WithFeed enables anomaly feed writes in the same transaction as the run.
func (h *RecordResultHandler) WithFeed(repo domainfeed.WriteRepository) *RecordResultHandler {
	h.feedRepo = repo
	return h
}

// Handle stores a result from `workflow.results` in one transaction: run, call
// and rule runs, certificates, workflow state (lease released) and the
// run.recorded.v1 event. A redelivered or late (timed out) result is a no-op.
func (h *RecordResultHandler) Handle(ctx context.Context, payload []byte) error {
	var result contract.Result
	if err := json.Unmarshal(payload, &result); err != nil {
		return messaging.NonRetryable(fmt.Errorf("decode result: %w", err))
	}
	if err := result.Validate(); err != nil {
		return messaging.NonRetryable(err)
	}
	r, certs := toRun(result)
	var notes []domainexpiry.NotificationRequest
	var incidentNotes []cmdincident.NotifyIntent
	var runNotify *contract.NotificationRequest
	var oneShotNotify *contract.NotificationRequest
	var recorded bool

	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		claimed, err := h.processed.MarkProcessed(txCtx, result.RunID, schedule.RecordRunHandlerName)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		state, err := h.states.GetForUpdate(txCtx, r.WorkflowID)
		if err != nil {
			return err
		}
		if state == nil {
			// The workflow was deleted while it ran: nothing to attach the run to.
			return nil
		}
		previousStatus := state.LastStatus
		if err := h.runs.Save(txCtx, r); err != nil {
			return err
		}
		if h.feedRepo != nil {
			if err := h.feedRepo.Append(txCtx, domainfeed.EventsFromRun(r)); err != nil {
				return err
			}
		}
		if err := h.certificates.Upsert(txCtx, certs); err != nil {
			return err
		}
		if h.expiry != nil {
			notes, err = h.expiry.Handle(txCtx, r.UserID, r.WorkflowID, observationsFromResult(result))
			if err != nil {
				return err
			}
		}

		// Incident evaluation is all-or-nothing with the run. An evaluation error
		// fails the whole message (retried) instead of silently dropping signals.
		if h.incidents != nil {
			if h.nestedTx == nil {
				return fmt.Errorf("incident evaluation: nested transaction not wired")
			}
			const sp = "incident_eval"
			if err := h.nestedTx.SavePoint(txCtx, sp); err != nil {
				return fmt.Errorf("incident savepoint: %w", err)
			}
			sigs := signalsFromRun(r)
			sigs = append(sigs, h.tlsSignals(txCtx, r.UserID, r.WorkflowID, observationsFromResult(result))...)
			applied, applyErr := h.incidents.Handle(txCtx, cmdincident.ApplySignalsCommand{
				UserID: r.UserID, WorkflowID: r.WorkflowID, Signals: sigs,
			})
			if applyErr != nil {
				if rbErr := h.nestedTx.RollbackTo(txCtx, sp); rbErr != nil {
					return fmt.Errorf("incident eval rollback: %w", rbErr)
				}
				return fmt.Errorf("incident evaluation: %w", applyErr)
			}
			incidentNotes = applied.Notifications
		}

		// Schedule and manual runs both hold a lease; release it on completion.
		if r.Trigger == contract.TriggerSchedule || r.Trigger == contract.TriggerManual {
			ownedLease := state.Running() && state.LeaseRunID != nil && *state.LeaseRunID == r.ID
			state.Complete(r.ID, string(r.Status), r.FinishedAt)
			// Only the scheduled fire that still owns the lease finishes a one-shot.
			// A late result from a previous interval run must not disable a rescheduled once/at.
			if r.Trigger == contract.TriggerSchedule && ownedLease {
				disabled, name, err := h.finishOneShot(txCtx, state, r.WorkflowID)
				if err != nil {
					return err
				}
				if disabled {
					oneShotNotify = &contract.NotificationRequest{
						V: contract.NotificationRequestV1, Kind: "workflow.one_shot.done",
						UserID: r.UserID.String(), WorkflowID: r.WorkflowID.String(),
						Subject: name, Severity: "info",
						Message: fmt.Sprintf("One-shot workflow %q finished and was disabled.", name),
					}
				}
			}
			if err := h.states.Save(txCtx, *state); err != nil {
				return err
			}
		}
		if note, ok := h.buildRunNotification(txCtx, r, previousStatus); ok {
			runNotify = &note
		}
		recorded = true
		return h.outbox.StoreEvents(txCtx, r.PullEvents())
	})
	if err != nil {
		return messaging.Retryable(fmt.Errorf("record run %s: %w", result.RunID, err))
	}
	if !recorded {
		return nil
	}
	h.publishExpiryNotes(ctx, notes)
	h.publishIncidentNotes(ctx, incidentNotes)
	if runNotify != nil {
		h.publishPrepared(ctx, *runNotify)
	}
	if oneShotNotify != nil {
		h.publishPrepared(ctx, *oneShotNotify)
	}
	return nil
}

func (h *RecordResultHandler) buildRunNotification(
	ctx context.Context, r *domainrun.Run, previousStatus string,
) (contract.NotificationRequest, bool) {
	if h.notifySettings == nil {
		return contract.NotificationRequest{}, false
	}
	status := string(r.Status)
	kind := ""
	severity := "critical"
	switch r.Status {
	case domainrun.StatusFailed:
		kind = "run.failed"
	case domainrun.StatusError:
		kind = "run.errored"
	case domainrun.StatusSuccess:
		kind = "run.success"
		severity = "info"
	default:
		return contract.NotificationRequest{}, false
	}
	settings, err := notifyresolve.Resolve(ctx, h.notifySettings, r.UserID, r.WorkflowID)
	if err != nil || !settings.ShouldNotifyRun(status, previousStatus) {
		return contract.NotificationRequest{}, false
	}
	msg := fmt.Sprintf("Workflow run finished with status %s", status)
	if r.Reason != "" {
		msg = fmt.Sprintf("Workflow run finished with status %s (%s)", status, r.Reason)
	}
	req := contract.NotificationRequest{
		V: contract.NotificationRequestV1, Kind: kind, Type: status,
		UserID: r.UserID.String(), WorkflowID: r.WorkflowID.String(),
		ItemID: r.ID.String(), Severity: severity, Message: msg,
		Recipients: append([]string(nil), settings.Emails...),
	}
	return req, true
}

func (h *RecordResultHandler) publishPrepared(ctx context.Context, req contract.NotificationRequest) {
	if h.notify == nil {
		return
	}
	if err := h.notify.PublishRequest(ctx, req); err != nil {
		h.logger.Error("publish notification failed", "kind", req.Kind, "workflow_id", req.WorkflowID, "error", err)
	}
}

func (h *RecordResultHandler) publishIncidentNotes(ctx context.Context, notes []cmdincident.NotifyIntent) {
	if h.notify == nil || len(notes) == 0 {
		return
	}
	for _, n := range notes {
		req := contract.NotificationRequest{
			V: contract.NotificationRequestV1, Kind: n.Kind,
			UserID: n.UserID.String(), WorkflowID: n.WorkflowID.String(),
			ItemID: n.IncidentID.String(), IncidentID: n.IncidentID.String(),
			Severity: string(n.Severity), Type: string(n.IncidentKind), Message: n.Message,
		}
		req, ok := notifyresolve.PrepareRequest(
			ctx, h.notifySettings, n.UserID, n.WorkflowID,
			notifyresolve.FamilyIncident, req, h.logger,
		)
		if !ok {
			continue
		}
		if err := h.notify.PublishRequest(ctx, req); err != nil {
			h.logger.Error("publish incident notification failed", "incident_id", n.IncidentID, "error", err)
		}
	}
}

func (h *RecordResultHandler) tlsSignals(
	ctx context.Context, userID, workflowID uuid.UUID, observations []domainexpiry.Observation,
) []domainincident.Signal {
	if h.expiryPolicies == nil || len(observations) == 0 {
		return nil
	}
	var out []domainincident.Signal
	for _, obs := range observations {
		policy, err := cmdexpiry.ResolvePolicy(ctx, h.expiryPolicies, userID, workflowID, obs.Type, h.clock.Now())
		if err != nil {
			h.logger.Error("tls incident policy resolve failed", "error", err)
			continue
		}
		daysLeft := domainexpiry.DaysLeft(obs.ExpiresAt, obs.ObservedAt)
		if sig := domainincident.TLSSignal(obs.Subject, daysLeft, policy.ThresholdsDays, obs.ObservedAt); sig != nil {
			out = append(out, *sig)
		}
	}
	return out
}

// signalsFromRun builds incident signals for run-based kinds.
func signalsFromRun(r *domainrun.Run) []domainincident.Signal {
	var out []domainincident.Signal
	runID := r.ID
	obs := r.StartedAt
	if obs.IsZero() {
		obs = r.FinishedAt
	}
	callsByName := map[string]uuid.UUID{}
	for _, c := range r.Calls {
		if c.CallID != nil && c.Name != "" {
			callsByName[c.Name] = *c.CallID
		}
	}
	for _, c := range r.Calls {
		if c.CallID == nil {
			continue
		}
		callID := *c.CallID
		subject := domainincident.CallSubject(callID)
		switch c.Status {
		case domainrun.CallSkipped:
			if strings.HasPrefix(c.Reason, "missing variable") {
				cause := map[string]any{"status": string(c.Status), "reason": c.Reason}
				if producer := producerCallIDFromReason(c.Reason, callsByName); producer != nil {
					cause["producerCallId"] = producer.String()
				}
				out = append(out, domainincident.Signal{
					Kind: domainincident.KindChainBroken, Failing: true,
					CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
					Cause: cause,
				})
			}
			// Dependency skips are neither fail nor success for call_failed/call_slow.
			continue
		case domainrun.CallFailed, domainrun.CallError:
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindCallFailed, Failing: true,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
				Cause: map[string]any{"status": string(c.Status), "reason": c.Reason},
			})
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindChainBroken, Failing: false,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
			})
		case domainrun.CallSuccess:
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindCallFailed, Failing: false,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
			})
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindChainBroken, Failing: false,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
			})
		}
		var slowFails []map[string]any
		slowSeen := false
		for _, rule := range c.Rules {
			if rule.Subject != "response_time" {
				continue
			}
			slowSeen = true
			if !rule.Passed {
				slowFails = append(slowFails, map[string]any{
					"subject": rule.Subject, "observed": rule.Observed, "expected": rule.Expected,
				})
			}
		}
		if len(slowFails) > 0 {
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindCallSlow, Failing: true,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
				Cause: map[string]any{"rules": slowFails},
			})
		} else if slowSeen {
			out = append(out, domainincident.Signal{
				Kind: domainincident.KindCallSlow, Failing: false,
				CallID: &callID, Subject: subject, RunID: &runID, ObservedAt: obs,
			})
		}
	}
	wfFailing := r.Status == domainrun.StatusFailed || r.Status == domainrun.StatusError
	wfHealthy := r.Status == domainrun.StatusSuccess
	if wfFailing || wfHealthy {
		out = append(out, domainincident.Signal{
			Kind: domainincident.KindWorkflowFailed, Failing: wfFailing,
			Subject: "", RunID: &runID, ObservedAt: obs,
			Cause: map[string]any{"status": string(r.Status)},
		})
	}
	return out
}

func producerCallIDFromReason(reason string, callsByName map[string]uuid.UUID) *uuid.UUID {
	const marker = `not extracted by "`
	idx := strings.Index(reason, marker)
	if idx < 0 {
		return nil
	}
	rest := reason[idx+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end <= 0 {
		return nil
	}
	name := rest[:end]
	id, ok := callsByName[name]
	if !ok {
		return nil
	}
	return &id
}

func (h *RecordResultHandler) publishExpiryNotes(ctx context.Context, notes []domainexpiry.NotificationRequest) {
	if h.notify == nil || len(notes) == 0 {
		return
	}
	for _, n := range notes {
		req := toContractNotifications([]domainexpiry.NotificationRequest{n})[0]
		req, ok := notifyresolve.PrepareRequest(
			ctx, h.notifySettings, n.UserID, n.WorkflowID,
			notifyresolve.FamilyTLS, req, h.logger,
		)
		if !ok {
			continue
		}
		if err := h.notify.PublishRequest(ctx, req); err != nil {
			h.logger.Error("publish expiry notification failed", "workflow_id", req.WorkflowID, "error", err)
		}
	}
}

// finishOneShot parks and disables once/at workflows after their scheduled fire.
// Returns whether the workflow was newly disabled and its name for notification.
func (h *RecordResultHandler) finishOneShot(ctx context.Context, state *domainschedule.State, workflowID uuid.UUID) (bool, string, error) {
	w, err := h.workflows.GetByID(ctx, workflowID)
	if err != nil {
		return false, "", err
	}
	if w == nil || !w.ScheduleType.IsOneShot() {
		return false, "", nil
	}
	state.ClearSchedule()
	if !w.Disable(h.clock.Now()) {
		return false, "", nil
	}
	if err := h.workflows.Update(ctx, w); err != nil {
		return false, "", err
	}
	if err := h.outbox.StoreEvents(ctx, w.PullEvents()); err != nil {
		return false, "", err
	}
	return true, w.Name, nil
}

func toRun(result contract.Result) (*domainrun.Run, []certificate.Certificate) {
	runID, _ := uuid.Parse(result.RunID)
	workflowID, _ := uuid.Parse(result.WorkflowID)
	userID, _ := uuid.Parse(result.UserID)

	r := &domainrun.Run{
		ID:         runID,
		WorkflowID: workflowID,
		UserID:     userID,
		Trigger:    result.Trigger,
		Status:     domainrun.Status(result.Status),
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		Counts: domainrun.Counts{
			Success: result.Counts.Success, Failed: result.Counts.Failed,
			Error: result.Counts.Error, Skipped: result.Counts.Skipped,
		},
		Variables: result.Variables,
	}

	var certs []certificate.Certificate
	seen := map[string]struct{}{}
	for _, c := range result.Calls {
		// One mapping for call runs, shared with the progress path (toCallRun).
		cr := toCallRun(c)
		r.Calls = append(r.Calls, cr)

		if c.TLS != nil && c.TLS.Host != "" && !c.TLS.ExpiresAt.IsZero() {
			key := fmt.Sprintf("%s:%d", c.TLS.Host, c.TLS.Port)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			seenAt := c.FinishedAt
			if seenAt.IsZero() {
				seenAt = time.Now().UTC()
			}
			certs = append(certs, certificate.Certificate{
				UserID: userID, Host: c.TLS.Host, Port: c.TLS.Port,
				Subject: c.TLS.Subject, Issuer: c.TLS.Issuer, SerialNumber: c.TLS.SerialNumber,
				ExpiresAt: c.TLS.ExpiresAt, ChainExpiresAt: c.TLS.ChainExpiresAt,
				ChainValid: c.TLS.ChainValid, NameMatch: c.TLS.NameMatch, SeenAt: seenAt,
			})
		}
	}
	return domainrun.Recorded(r), certs
}
