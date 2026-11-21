package feed

import (
	"fmt"
	"time"

	domainrun "go-api/internal/domain/run"

	"github.com/google/uuid"
)

// EventsFromRun builds anomaly feed rows for one stored run (no successes).
func EventsFromRun(r *domainrun.Run) []Event {
	if r == nil {
		return nil
	}
	wfID := r.WorkflowID
	runID := r.ID
	at := r.FinishedAt
	if at.IsZero() {
		at = r.StartedAt
	}
	var out []Event

	for _, c := range r.Calls {
		switch c.Status {
		case domainrun.CallFailed:
			out = append(out, callEvent(r, c, KindCallFailed, SeverityCritical, callFailedTitle(c), callPayload(c)))
		case domainrun.CallError:
			out = append(out, callEvent(r, c, KindCallErrored, SeverityCritical, callErroredTitle(c), callPayload(c)))
		}
		for _, rule := range c.Rules {
			if rule.Subject != "response_time" || rule.Passed {
				continue
			}
			payload := callPayload(c)
			payload["observed"] = rule.Observed
			payload["expected"] = rule.Expected
			out = append(out, callEvent(r, c, KindCallSlow, SeverityWarning, callSlowTitle(c, rule), payload))
			break
		}
	}

	switch r.Status {
	case domainrun.StatusSkipped:
		out = append(out, Event{
			ID: uuid.New(), UserID: r.UserID, WorkflowID: &wfID, RunID: &runID,
			Kind: KindRunSkipped, Severity: SeverityInfo, At: at.UTC(),
			Title:   "Run skipped (overlap)",
			Payload: map[string]any{"reason": r.Reason, "status": string(r.Status)},
		})
	case domainrun.StatusError:
		if r.Reason == domainrun.ReasonTimeout {
			out = append(out, Event{
				ID: uuid.New(), UserID: r.UserID, WorkflowID: &wfID, RunID: &runID,
				Kind: KindRunTimedOut, Severity: SeverityCritical, At: at.UTC(),
				Title:   "Run timed out",
				Payload: map[string]any{"reason": r.Reason, "status": string(r.Status)},
			})
			break
		}
		out = append(out, Event{
			ID: uuid.New(), UserID: r.UserID, WorkflowID: &wfID, RunID: &runID,
			Kind: KindRunErrored, Severity: SeverityCritical, At: at.UTC(),
			Title:   "Run errored",
			Payload: map[string]any{"reason": r.Reason, "status": string(r.Status)},
		})
	case domainrun.StatusFailed:
		out = append(out, Event{
			ID: uuid.New(), UserID: r.UserID, WorkflowID: &wfID, RunID: &runID,
			Kind: KindRunFailed, Severity: SeverityCritical, At: at.UTC(),
			Title:   "Run failed",
			Payload: map[string]any{"reason": r.Reason, "status": string(r.Status)},
		})
	}
	return out
}

func callEvent(r *domainrun.Run, c domainrun.CallRun, kind Kind, sev Severity, title string, payload map[string]any) Event {
	wfID := r.WorkflowID
	runID := r.ID
	finished := c.FinishedAt
	if finished.IsZero() {
		finished = r.FinishedAt
	}
	if finished.IsZero() {
		finished = r.StartedAt
	}
	ev := Event{
		ID: uuid.New(), UserID: r.UserID, WorkflowID: &wfID, RunID: &runID,
		Kind: kind, Severity: sev, At: finished.UTC(), Title: title, Payload: payload,
	}
	if c.CallID != nil {
		id := *c.CallID
		ev.CallID = &id
	}
	return ev
}

func callPayload(c domainrun.CallRun) map[string]any {
	p := map[string]any{
		"name":     c.Name,
		"position": c.Position,
		"status":   string(c.Status),
		"reason":   c.Reason,
	}
	if c.HTTPStatus != 0 {
		p["httpStatus"] = c.HTTPStatus
	}
	if c.Timings.Total > 0 {
		p["latencyMs"] = c.Timings.Total
	}
	if c.Error != nil {
		p["error"] = c.Error.Message
	}
	return p
}

func callFailedTitle(c domainrun.CallRun) string {
	name := c.Name
	if name == "" {
		name = "Call"
	}
	if c.HTTPStatus != 0 {
		return fmt.Sprintf("%s failed (%d)", name, c.HTTPStatus)
	}
	return fmt.Sprintf("%s failed", name)
}

func callErroredTitle(c domainrun.CallRun) string {
	name := c.Name
	if name == "" {
		name = "Call"
	}
	if c.Error != nil && c.Error.Message != "" {
		return fmt.Sprintf("%s errored: %s", name, c.Error.Message)
	}
	return fmt.Sprintf("%s errored", name)
}

func callSlowTitle(c domainrun.CallRun, rule domainrun.RuleRun) string {
	name := c.Name
	if name == "" {
		name = "Call"
	}
	return fmt.Sprintf("%s slow (%s, expected %s)", name, rule.Observed, rule.Expected)
}

// TLSThresholdEvent builds a feed row for one newly claimed TLS threshold.
func TLSThresholdEvent(userID, workflowID uuid.UUID, subject string, daysLeft, thresholdDays int, at time.Time) Event {
	wf := workflowID
	sev := SeverityWarning
	kind := KindTLSThresholdCrossed
	title := fmt.Sprintf("TLS %s: %d days left", subject, daysLeft)
	if daysLeft < 0 {
		kind = KindTLSExpired
		sev = SeverityCritical
		title = fmt.Sprintf("TLS %s expired", subject)
	}
	return Event{
		ID: uuid.New(), UserID: userID, WorkflowID: &wf,
		Kind: kind, Severity: sev, At: at.UTC(), Title: title,
		Payload: map[string]any{
			"subject": subject, "daysLeft": daysLeft, "thresholdDays": thresholdDays,
		},
	}
}

// AvailabilityEvent builds breached/recovered feed rows.
func AvailabilityEvent(userID, workflowID uuid.UUID, failing bool, ratio, threshold float64, incidentID *uuid.UUID, at time.Time) Event {
	wf := workflowID
	kind := KindAvailabilityRecovered
	sev := SeveritySuccess
	title := fmt.Sprintf("Availability recovered (%.1f%%)", ratio)
	if failing {
		kind = KindAvailabilityBreached
		sev = SeverityCritical
		title = fmt.Sprintf("Availability breached (%.1f%% < %.1f%%)", ratio, threshold)
	}
	return Event{
		ID: uuid.New(), UserID: userID, WorkflowID: &wf, IncidentID: incidentID,
		Kind: kind, Severity: sev, At: at.UTC(), Title: title,
		Payload: map[string]any{"ratio": ratio, "threshold": threshold},
	}
}
