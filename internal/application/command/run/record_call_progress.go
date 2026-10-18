package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"

	"github.com/google/uuid"
)

const RecordCallProgressHandlerName = "record_call_progress"

type RecordCallProgressHandler struct {
	states    domainschedule.Repository
	runs      domainrun.WriteRepository
	processed port.ProcessedEventRepository
	outbox    port.OutboxRepository
}

func NewRecordCallProgressHandler(
	states domainschedule.Repository,
	runs domainrun.WriteRepository,
	processed port.ProcessedEventRepository,
	outbox port.OutboxRepository,
) *RecordCallProgressHandler {
	return &RecordCallProgressHandler{
		states: states, runs: runs, processed: processed, outbox: outbox,
	}
}

// Handle persists progressive call progress and emits realtime-oriented domain events.
// phase=started: ensure running run + outbox call.started (and run.started when created).
// phase=finished: upsert call_run + outbox call.finished (and run.started when created).
func (h *RecordCallProgressHandler) Handle(ctx context.Context, payload []byte) error {
	var progress contract.CallProgress
	if err := json.Unmarshal(payload, &progress); err != nil {
		return messaging.NonRetryable(fmt.Errorf("decode call progress: %w", err))
	}
	if err := progress.Validate(); err != nil {
		return messaging.NonRetryable(err)
	}

	runID, _ := uuid.Parse(progress.RunID)
	workflowID, _ := uuid.Parse(progress.WorkflowID)
	userID, _ := uuid.Parse(progress.UserID)
	call := toCallRun(progress.Call)
	startedAt := call.StartedAt
	if startedAt.IsZero() {
		startedAt = progress.EmittedAt
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	phase := progress.NormalizedPhase()

	r := &domainrun.Run{
		ID:         runID,
		WorkflowID: workflowID,
		UserID:     userID,
		Trigger:    progress.Trigger,
		Status:     domainrun.StatusRunning,
		StartedAt:  startedAt,
		Variables:  map[string]string{},
	}

	err := h.states.WithTransaction(ctx, func(txCtx context.Context) error {
		claimed, err := h.processed.MarkProcessed(txCtx, progress.ID, RecordCallProgressHandlerName)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		created, err := h.runs.EnsureRunning(txCtx, r)
		if err != nil {
			return err
		}
		if created {
			r.Started()
		}
		switch phase {
		case contract.CallProgressPhaseStarted:
			r.CallStarted(call)
		default:
			r.CallFinished(call)
			if err := h.runs.UpsertCallRun(txCtx, r, call); err != nil {
				return err
			}
		}
		return h.outbox.StoreEvents(txCtx, r.PullEvents())
	})
	if err != nil {
		return messaging.Retryable(fmt.Errorf("record call progress %s: %w", progress.ID, err))
	}
	return nil
}

func toCallRun(c contract.CallRun) domainrun.CallRun {
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
	return cr
}
