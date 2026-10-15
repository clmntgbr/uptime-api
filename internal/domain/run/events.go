package run

import "time"

const (
	EventTypeRunRecorded   = "run.recorded.v1"
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

// RunProgressed is emitted when a call_run is persisted while the run is still open.
type RunProgressed struct {
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

func (e RunProgressed) EventID() string       { return e.ID }
func (e RunProgressed) EventType() string     { return EventTypeRunProgressed }
func (e RunProgressed) AggregateID() string   { return e.WorkflowID }
func (e RunProgressed) OccurredAt() time.Time { return e.Timestamp }
