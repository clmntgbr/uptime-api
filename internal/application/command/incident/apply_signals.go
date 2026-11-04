package incident

import (
	"context"
	"fmt"
	"time"

	"go-api/internal/domain/event"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

// ApplyResult is returned after signals are applied (publish after commit).
type ApplyResult struct {
	Notifications []NotifyIntent
	DomainEvents  []event.DomainEvent
}

type NotifyIntent struct {
	Kind       string // incident.opened | incident.resolved | incident.reminder
	IncidentID uuid.UUID
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	IncidentKind domainincident.Kind
	Severity   domainincident.Severity
	Message    string
}

type ApplySignalsCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Signals    []domainincident.Signal
}

type ApplySignalsHandler struct {
	policies  domainincident.PolicyRepository
	states    domainincident.SignalStateRepository
	incidents domainincident.IncidentRepository
	events    domainincident.EventRepository
	outbox    port.OutboxRepository
	clock     func() time.Time
}

func NewApplySignalsHandler(
	policies domainincident.PolicyRepository,
	states domainincident.SignalStateRepository,
	incidents domainincident.IncidentRepository,
	events domainincident.EventRepository,
	outbox port.OutboxRepository,
) *ApplySignalsHandler {
	return &ApplySignalsHandler{
		policies: policies, states: states, incidents: incidents,
		events: events, outbox: outbox, clock: time.Now,
	}
}

func (h *ApplySignalsHandler) Handle(ctx context.Context, cmd ApplySignalsCommand) (ApplyResult, error) {
	var result ApplyResult
	now := h.clock().UTC()
	for _, sig := range cmd.Signals {
		policy, err := h.resolvePolicy(ctx, cmd.UserID, cmd.WorkflowID, sig.Kind)
		if err != nil {
			return result, err
		}
		if !policy.Enabled {
			continue
		}
		if sig.Kind == domainincident.KindChainBroken && sig.Failing {
			skip, err := h.producerIncidentOpen(ctx, cmd.WorkflowID, sig.Cause)
			if err != nil {
				return result, err
			}
			if skip {
				continue
			}
		}
		severity := policy.Severity
		if sig.Severity != nil {
			severity = *sig.Severity
		}
		fp := domainincident.Fingerprint(cmd.WorkflowID, sig.Kind, sig.Subject)
		state, err := h.states.LockGetOrCreate(ctx, cmd.WorkflowID, fp)
		if err != nil {
			return result, err
		}
		if domainincident.ShouldSkipSignal(state, sig.RunID, sig.ObservedAt) {
			continue
		}
		failures, successes := domainincident.NextCounters(sig.Failing, state.ConsecutiveFailures, state.ConsecutiveSuccesses)
		state.ConsecutiveFailures = failures
		state.ConsecutiveSuccesses = successes
		state.LastRunID = sig.RunID
		obs := sig.ObservedAt.UTC()
		state.LastSignalAt = &obs
		state.UpdatedAt = now
		if err := h.states.Save(ctx, state); err != nil {
			return result, err
		}

		active, err := h.incidents.FindActive(ctx, cmd.WorkflowID, fp)
		if err != nil {
			return result, err
		}

		switch {
		case sig.Failing && active == nil:
			resolved, err := h.incidents.FindLatestResolved(ctx, cmd.WorkflowID, fp)
			if err != nil {
				return result, err
			}
			if resolved != nil && domainincident.ShouldReopen(resolved.ResolvedAt, now) {
				resolved.ApplyReopen(now, sig.Cause, sig.RunID, severity)
				if err := h.incidents.Update(ctx, *resolved); err != nil {
					return result, err
				}
				if err := h.appendEvent(ctx, resolved.ID, domainincident.EventReopened, sig.RunID, sig.Cause, now); err != nil {
					return result, err
				}
				ev := domainincident.NewReopenedEvent(*resolved, now)
				result.DomainEvents = append(result.DomainEvents, ev)
				result.Notifications = append(result.Notifications, notifyOpened(*resolved))
				continue
			}
			if !domainincident.ShouldOpen(failures, policy.OpenAfter) {
				continue
			}
			inc := domainincident.Incident{
				ID: uuid.New(), UserID: cmd.UserID, WorkflowID: cmd.WorkflowID,
				Kind: sig.Kind, CallID: sig.CallID, Fingerprint: fp,
				Severity: severity, State: domainincident.StateOpen,
				Cause: sig.Cause, OpenedAt: now, OpenedByRunID: sig.RunID,
				LastFailureRunID: sig.RunID,
			}
			if err := h.incidents.Insert(ctx, inc); err != nil {
				return result, err
			}
			if err := h.appendEvent(ctx, inc.ID, domainincident.EventOpened, sig.RunID, sig.Cause, now); err != nil {
				return result, err
			}
			result.DomainEvents = append(result.DomainEvents, domainincident.NewOpenedEvent(inc, now))
			result.Notifications = append(result.Notifications, notifyOpened(inc))

		case sig.Failing && active != nil:
			active.Cause = sig.Cause
			active.LastFailureRunID = sig.RunID
			active.Severity = severity
			if err := h.incidents.Update(ctx, *active); err != nil {
				return result, err
			}
			if err := h.appendEvent(ctx, active.ID, domainincident.EventFailing, sig.RunID, sig.Cause, now); err != nil {
				return result, err
			}

		case !sig.Failing && active != nil:
			if !domainincident.ShouldClose(successes, policy.CloseAfter) {
				continue
			}
			active.Resolve(now)
			if err := h.incidents.Update(ctx, *active); err != nil {
				return result, err
			}
			if err := h.appendEvent(ctx, active.ID, domainincident.EventResolved, sig.RunID, map[string]any{}, now); err != nil {
				return result, err
			}
			result.DomainEvents = append(result.DomainEvents, domainincident.NewResolvedEvent(*active, now))
			result.Notifications = append(result.Notifications, NotifyIntent{
				Kind: "incident.resolved", IncidentID: active.ID,
				UserID: active.UserID, WorkflowID: active.WorkflowID,
				IncidentKind: active.Kind, Severity: active.Severity,
				Message: fmt.Sprintf("Incident %s resolved", active.Kind),
			})
		}
	}
	if len(result.DomainEvents) > 0 && h.outbox != nil {
		if err := h.outbox.StoreEvents(ctx, result.DomainEvents); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (h *ApplySignalsHandler) resolvePolicy(ctx context.Context, userID, workflowID uuid.UUID, kind domainincident.Kind) (domainincident.Policy, error) {
	p, err := h.policies.Get(ctx, workflowID, kind)
	if err != nil {
		return domainincident.Policy{}, err
	}
	if p == nil {
		return domainincident.DefaultPolicy(userID, workflowID, kind), nil
	}
	return *p, nil
}

// producerIncidentOpen is true when a chain_broken cause points at a producer
// call that already has an active call_failed incident.
func (h *ApplySignalsHandler) producerIncidentOpen(ctx context.Context, workflowID uuid.UUID, cause map[string]any) (bool, error) {
	if cause == nil {
		return false, nil
	}
	raw, ok := cause["producerCallId"]
	if !ok || raw == nil {
		return false, nil
	}
	var producerID uuid.UUID
	switch v := raw.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return false, nil
		}
		producerID = id
	case uuid.UUID:
		producerID = v
	default:
		return false, nil
	}
	fp := domainincident.Fingerprint(workflowID, domainincident.KindCallFailed, domainincident.CallSubject(producerID))
	active, err := h.incidents.FindActive(ctx, workflowID, fp)
	if err != nil {
		return false, err
	}
	return active != nil, nil
}

func (h *ApplySignalsHandler) appendEvent(ctx context.Context, incidentID uuid.UUID, typ domainincident.EventType, runID *uuid.UUID, payload map[string]any, at time.Time) error {
	if payload == nil {
		payload = map[string]any{}
	}
	return h.events.Append(ctx, domainincident.Event{
		ID: uuid.New(), IncidentID: incidentID, At: at, Type: typ, RunID: runID, Payload: payload,
	})
}

func notifyOpened(inc domainincident.Incident) NotifyIntent {
	return NotifyIntent{
		Kind: "incident.opened", IncidentID: inc.ID,
		UserID: inc.UserID, WorkflowID: inc.WorkflowID,
		IncidentKind: inc.Kind, Severity: inc.Severity,
		Message: fmt.Sprintf("Incident %s opened", inc.Kind),
	}
}
