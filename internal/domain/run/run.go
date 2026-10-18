package run

import (
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/event"

	"github.com/google/uuid"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
	StatusError   Status = "error"
)

const (
	ReasonOverlap = "overlap"
	ReasonTimeout = "timeout"
)

type CallStatus string

const (
	CallSuccess CallStatus = "success"
	CallFailed  CallStatus = "failed"
	CallError   CallStatus = "error"
	CallSkipped CallStatus = "skipped"
)

type Counts struct {
	Success int
	Failed  int
	Error   int
	Skipped int
}

func (c Counts) Total() int { return c.Success + c.Failed + c.Error + c.Skipped }

// Run is one pass of a workflow with the detail of every call. The request,
// response, timing, network and TLS details reuse the wire contract shapes:
// they are stored as they were observed.
type Run struct {
	ID         uuid.UUID
	WorkflowID uuid.UUID
	UserID     uuid.UUID
	Trigger    string
	Status     Status
	Reason     string
	StartedAt  time.Time
	FinishedAt time.Time
	Counts     Counts
	Variables  map[string]string
	Calls      []CallRun

	events []event.DomainEvent
}

type CallRun struct {
	ID                 uuid.UUID
	CallID             *uuid.UUID
	Name               string
	Position           int
	Status             CallStatus
	Reason             string
	HTTPStatus         int
	Request            contract.RequestInfo
	Response           contract.ResponseInfo
	Timings            contract.Timings
	Network            contract.NetworkInfo
	TLS                *contract.TLSInfo
	Rules              []RuleRun
	ExtractedVariables map[string]string
	UsedVariables      []string
	Error              *contract.CallError
	BodyRef            string
	BodyHash           string
	BodyEncoding       string
	ResponseKeys       []string
	StartedAt          time.Time
	FinishedAt         time.Time
}

type RuleRun struct {
	ID       uuid.UUID
	Position int
	Subject  string
	Operator string
	Text     string
	Observed string
	Expected string
	Passed   bool
}

func (r *Run) Duration() time.Duration {
	if r.FinishedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// Started records that this run row was created as status=running.
func (r *Run) Started() {
	r.events = append(r.events, RunStarted{
		ID:         uuid.New().String(),
		RunID:      r.ID.String(),
		WorkflowID: r.WorkflowID.String(),
		UserID:     r.UserID.String(),
		Trigger:    r.Trigger,
		StartedAt:  r.StartedAt,
		Timestamp:  time.Now().UTC(),
	})
}

// CallStarted records that a call began executing inside this open run.
func (r *Run) CallStarted(call CallRun) {
	callID := ""
	if call.CallID != nil {
		callID = call.CallID.String()
	}
	r.events = append(r.events, CallStarted{
		ID:         uuid.New().String(),
		RunID:      r.ID.String(),
		WorkflowID: r.WorkflowID.String(),
		UserID:     r.UserID.String(),
		CallID:     callID,
		CallName:   call.Name,
		Position:   call.Position,
		Timestamp:  time.Now().UTC(),
	})
}

// CallFinished records a realtime-oriented event for one finished call.
func (r *Run) CallFinished(call CallRun) {
	callID := ""
	if call.CallID != nil {
		callID = call.CallID.String()
	}
	r.events = append(r.events, CallFinished{
		ID:         uuid.New().String(),
		RunID:      r.ID.String(),
		WorkflowID: r.WorkflowID.String(),
		UserID:     r.UserID.String(),
		CallID:     callID,
		CallName:   call.Name,
		Position:   call.Position,
		Status:     string(call.Status),
		Timestamp:  time.Now().UTC(),
	})
}

// Recorded finalizes a run built from an execution result and records the event.
func Recorded(r *Run) *Run {
	r.recordEvent()
	return r
}

// NewSkipped records a pass that did not execute because a run was still in progress.
func NewSkipped(workflowID, userID uuid.UUID, at time.Time) *Run {
	r := &Run{
		ID:         uuid.New(),
		WorkflowID: workflowID,
		UserID:     userID,
		Trigger:    contract.TriggerSchedule,
		Status:     StatusSkipped,
		Reason:     ReasonOverlap,
		StartedAt:  at,
		FinishedAt: at,
	}
	r.recordEvent()
	return r
}

// NewTimedOut records a run whose lease expired before any result arrived.
func NewTimedOut(runID, workflowID, userID uuid.UUID, startedAt, expiredAt time.Time) *Run {
	r := &Run{
		ID:         runID,
		WorkflowID: workflowID,
		UserID:     userID,
		Trigger:    contract.TriggerSchedule,
		Status:     StatusError,
		Reason:     ReasonTimeout,
		StartedAt:  startedAt,
		FinishedAt: expiredAt,
	}
	r.recordEvent()
	return r
}

func (r *Run) PullEvents() []event.DomainEvent {
	events := r.events
	r.events = nil
	return events
}

func (r *Run) recordEvent() {
	r.events = append(r.events, RunRecorded{
		ID:         uuid.New().String(),
		RunID:      r.ID.String(),
		WorkflowID: r.WorkflowID.String(),
		UserID:     r.UserID.String(),
		Trigger:    r.Trigger,
		Status:     string(r.Status),
		Reason:     r.Reason,
		Success:    r.Counts.Success,
		Failed:     r.Counts.Failed,
		Error:      r.Counts.Error,
		Skipped:    r.Counts.Skipped,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Timestamp:  time.Now().UTC(),
	})
}
