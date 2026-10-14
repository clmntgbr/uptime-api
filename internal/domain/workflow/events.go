package workflow

import "time"

const (
	EventTypeWorkflowCreated = "workflow.created.v1"
	EventTypeWorkflowUpdated = "workflow.updated.v1"
	EventTypeWorkflowDeleted = "workflow.deleted.v1"
)

type WorkflowCreated struct {
	ID              string     `json:"eventId"`
	WorkflowID      string     `json:"workflowId"`
	UserID          string     `json:"userId"`
	Name            string     `json:"name"`
	ScheduleType    string     `json:"scheduleType"`
	IntervalSeconds int        `json:"intervalSeconds"`
	RunAt           *time.Time `json:"runAt,omitempty"`
	Enabled         bool       `json:"enabled"`
	CallCount       int        `json:"callCount"`
	Timestamp       time.Time  `json:"timestamp"`
}

func (e WorkflowCreated) EventID() string       { return e.ID }
func (e WorkflowCreated) EventType() string     { return EventTypeWorkflowCreated }
func (e WorkflowCreated) AggregateID() string   { return e.WorkflowID }
func (e WorkflowCreated) OccurredAt() time.Time { return e.Timestamp }

type WorkflowUpdated struct {
	ID              string     `json:"eventId"`
	WorkflowID      string     `json:"workflowId"`
	UserID          string     `json:"userId"`
	Name            string     `json:"name"`
	ScheduleType    string     `json:"scheduleType"`
	IntervalSeconds int        `json:"intervalSeconds"`
	RunAt           *time.Time `json:"runAt,omitempty"`
	Enabled         bool       `json:"enabled"`
	CallCount       int        `json:"callCount"`
	Timestamp       time.Time  `json:"timestamp"`
}

func (e WorkflowUpdated) EventID() string       { return e.ID }
func (e WorkflowUpdated) EventType() string     { return EventTypeWorkflowUpdated }
func (e WorkflowUpdated) AggregateID() string   { return e.WorkflowID }
func (e WorkflowUpdated) OccurredAt() time.Time { return e.Timestamp }

type WorkflowDeleted struct {
	ID         string    `json:"eventId"`
	WorkflowID string    `json:"workflowId"`
	UserID     string    `json:"userId"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e WorkflowDeleted) EventID() string       { return e.ID }
func (e WorkflowDeleted) EventType() string     { return EventTypeWorkflowDeleted }
func (e WorkflowDeleted) AggregateID() string   { return e.WorkflowID }
func (e WorkflowDeleted) OccurredAt() time.Time { return e.Timestamp }
