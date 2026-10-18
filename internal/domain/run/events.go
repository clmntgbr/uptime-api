package run

import "time"

const (
	EventTypeRunRecorded  = "run.recorded.v1"
	EventTypeRunStarted   = "run.started.v1"
	EventTypeCallStarted  = "call.started.v1"
	EventTypeCallFinished = "call.finished.v1"
	// EventTypeRunProgressed is the legacy alias of call.finished.v1. Kept so
	// unpublished outbox rows from a previous deploy still reach Centrifugo.
	EventTypeRunProgressed = "run.progressed.v1"
)

// RunRecorded is emitted once a run (executed, skipped or timed out) is stored.
// It carries identifiers, status and counters only: never bodies or variables.
type RunRecorded struct {
	ID         string    `json:"eventId"`
	RunID      string    `json:"runId"`
	WorkflowID string    `json:"workflowId"`
	UserID     string    `json:"userId"`
	Trigger    string    `json:"trigger"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	Success    int       `json:"success"`
	Failed     int       `json:"failed"`
	Error      int       `json:"error"`
	Skipped    int       `json:"skipped"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e RunRecorded) EventID() string       { return e.ID }
func (e RunRecorded) EventType() string     { return EventTypeRunRecorded }
func (e RunRecorded) AggregateID() string   { return e.WorkflowID }
func (e RunRecorded) OccurredAt() time.Time { return e.Timestamp }

// RunStarted is emitted when a running run row is first created.
type RunStarted struct {
	ID         string    `json:"eventId"`
	RunID      string    `json:"runId"`
	WorkflowID string    `json:"workflowId"`
	UserID     string    `json:"userId"`
	Trigger    string    `json:"trigger"`
	StartedAt  time.Time `json:"startedAt"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e RunStarted) EventID() string       { return e.ID }
func (e RunStarted) EventType() string     { return EventTypeRunStarted }
func (e RunStarted) AggregateID() string   { return e.WorkflowID }
func (e RunStarted) OccurredAt() time.Time { return e.Timestamp }

// CallStarted is emitted when a call begins executing inside an open run.
type CallStarted struct {
	ID         string    `json:"eventId"`
	RunID      string    `json:"runId"`
	WorkflowID string    `json:"workflowId"`
	UserID     string    `json:"userId"`
	CallID     string    `json:"callId,omitempty"`
	CallName   string    `json:"callName"`
	Position   int       `json:"position"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e CallStarted) EventID() string       { return e.ID }
func (e CallStarted) EventType() string     { return EventTypeCallStarted }
func (e CallStarted) AggregateID() string   { return e.WorkflowID }
func (e CallStarted) OccurredAt() time.Time { return e.Timestamp }

// CallFinished is emitted when a call_run is persisted while the run is still open.
type CallFinished struct {
	ID         string    `json:"eventId"`
	RunID      string    `json:"runId"`
	WorkflowID string    `json:"workflowId"`
	UserID     string    `json:"userId"`
	CallID     string    `json:"callId,omitempty"`
	CallName   string    `json:"callName"`
	Position   int       `json:"position"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e CallFinished) EventID() string       { return e.ID }
func (e CallFinished) EventType() string     { return EventTypeCallFinished }
func (e CallFinished) AggregateID() string   { return e.WorkflowID }
func (e CallFinished) OccurredAt() time.Time { return e.Timestamp }
