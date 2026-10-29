package run

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	cmdexpiry "go-api/internal/application/command/expiry"
	"go-api/internal/application/command/schedule"
	"go-api/internal/application/messaging"
	"go-api/internal/contract"
	"go-api/internal/domain/certificate"
	domainexpiry "go-api/internal/domain/expiry"
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
	states       domainschedule.Repository
	workflows    domainworkflow.WriteRepository
	runs         domainrun.WriteRepository
	certificates certificate.Repository
	expiry       *cmdexpiry.RecordObservationsHandler
	notify       notificationPublisher
	processed    port.ProcessedEventRepository
	outbox       port.OutboxRepository
	logger       *slog.Logger
}

func NewRecordResultHandler(
	states domainschedule.Repository,
	workflows domainworkflow.WriteRepository,
	runs domainrun.WriteRepository,
	certificates certificate.Repository,
	expiry *cmdexpiry.RecordObservationsHandler,
	notify notificationPublisher,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
	logger *slog.Logger,
) *RecordResultHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecordResultHandler{
		states: states, workflows: workflows, runs: runs,
		certificates: certificates, expiry: expiry, notify: notify,
		processed: processed, outbox: outbox, logger: logger,
	}
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
		if err := h.runs.Save(txCtx, r); err != nil {
			return err
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

		// Schedule and manual runs both hold a lease; release it on completion.
		if r.Trigger == contract.TriggerSchedule || r.Trigger == contract.TriggerManual {
			ownedLease := state.Running() && state.LeaseRunID != nil && *state.LeaseRunID == r.ID
			state.Complete(r.ID, string(r.Status), r.FinishedAt)
			// Only the scheduled fire that still owns the lease finishes a one-shot.
			// A late result from a previous interval run must not disable a rescheduled once/at.
			if r.Trigger == contract.TriggerSchedule && ownedLease {
				if err := h.finishOneShot(txCtx, state, r.WorkflowID); err != nil {
					return err
				}
			}
			if err := h.states.Save(txCtx, *state); err != nil {
				return err
			}
		}
		return h.outbox.StoreEvents(txCtx, r.PullEvents())
	})
	if err != nil {
		return messaging.Retryable(fmt.Errorf("record run %s: %w", result.RunID, err))
	}
	h.publishExpiryNotes(ctx, notes)
	return nil
}

func (h *RecordResultHandler) publishExpiryNotes(ctx context.Context, notes []domainexpiry.NotificationRequest) {
	if h.notify == nil || len(notes) == 0 {
		return
	}
	for _, req := range toContractNotifications(notes) {
		if err := h.notify.PublishRequest(ctx, req); err != nil {
			h.logger.Error("publish expiry notification failed", "workflow_id", req.WorkflowID, "error", err)
		}
	}
}

// finishOneShot parks and disables once/at workflows after their scheduled fire.
func (h *RecordResultHandler) finishOneShot(ctx context.Context, state *domainschedule.State, workflowID uuid.UUID) error {
	w, err := h.workflows.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}
	if w == nil || !w.ScheduleType.IsOneShot() {
		return nil
	}
	state.ClearSchedule()
	if !w.Disable() {
		return nil
	}
	if err := h.workflows.Update(ctx, w); err != nil {
		return err
	}
	return h.outbox.StoreEvents(ctx, w.PullEvents())
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
		cr := domainrun.CallRun{
			ID:                 uuid.New(),
			Name:               c.Name,
			Position:           c.Order,
			Status:             domainrun.CallStatus(c.Status),
			Reason:             c.Reason,
			HTTPStatus:         c.HTTPStatus,
			Request:            c.Request,
			Response:           c.Response,
			Timings:            c.TimingsMS,
			Network:            c.Network,
			TLS:                c.TLS,
			ExtractedVariables: c.ExtractedVariables,
			UsedVariables:      c.UsedVariables,
			Error:              c.Error,
			BodyRef:            c.BodyRef,
			BodyHash:           c.BodyHash,
			BodyEncoding:       c.BodyEncoding,
			ResponseKeys:       c.Response.Keys,
			StartedAt:          c.StartedAt,
			FinishedAt:         c.FinishedAt,
		}
		if id, err := uuid.Parse(c.CallID); err == nil {
			cr.CallID = &id
		}
		for i, rr := range c.Rules {
			cr.Rules = append(cr.Rules, domainrun.RuleRun{
				ID: uuid.New(), Position: i + 1, Subject: rr.Subject, Operator: rr.Operator,
				Text: rr.Text, Observed: rr.Observed, Expected: rr.Expected, Passed: rr.Passed,
			})
		}
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
