package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/application/command/schedule"
	"go-api/internal/application/messaging"
	"go-api/internal/contract"
	"go-api/internal/domain/certificate"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"

	"github.com/google/uuid"
)

type RecordResultHandler struct {
	states       domainschedule.Repository
	runs         domainrun.WriteRepository
	certificates certificate.Repository
	processed    port.ProcessedEventRepository
	outbox       port.OutboxRepository
}

func NewRecordResultHandler(
	states domainschedule.Repository,
	runs domainrun.WriteRepository,
	certificates certificate.Repository,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
) *RecordResultHandler {
	return &RecordResultHandler{states: states, runs: runs, certificates: certificates, processed: processed, outbox: outbox}
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
		if r.Trigger == contract.TriggerSchedule {
			state.Complete(r.ID, string(r.Status), r.FinishedAt)
			if err := h.states.Save(txCtx, *state); err != nil {
				return err
			}
		}
		return h.outbox.StoreEvents(txCtx, r.PullEvents())
	})
	if err != nil {
		return messaging.Retryable(fmt.Errorf("record run %s: %w", result.RunID, err))
	}
	return nil
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
