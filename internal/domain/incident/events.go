package incident

import (
	"time"

	"go-api/internal/domain/event"

	"github.com/google/uuid"
)

const (
	EventTypeOpened       = "incident.opened.v1"
	EventTypeAcknowledged = "incident.acknowledged.v1"
	EventTypeResolved     = "incident.resolved.v1"
	EventTypeReopened     = "incident.reopened.v1"
)

type Opened struct {
	ID         string    `json:"eventId"`
	IncidentID string    `json:"incidentId"`
	UserID     string    `json:"userId"`
	WorkflowID string    `json:"workflowId"`
	Kind       string    `json:"kind"`
	Timestamp  time.Time `json:"occurredAt"`
}

func (e Opened) EventID() string           { return e.ID }
func (e Opened) EventType() string         { return EventTypeOpened }
func (e Opened) AggregateID() string       { return e.IncidentID }
func (e Opened) OccurredAt() time.Time     { return e.Timestamp }

type Acknowledged struct {
	ID         string    `json:"eventId"`
	IncidentID string    `json:"incidentId"`
	UserID     string    `json:"userId"`
	WorkflowID string    `json:"workflowId"`
	Timestamp  time.Time `json:"occurredAt"`
}

func (e Acknowledged) EventID() string       { return e.ID }
func (e Acknowledged) EventType() string     { return EventTypeAcknowledged }
func (e Acknowledged) AggregateID() string   { return e.IncidentID }
func (e Acknowledged) OccurredAt() time.Time { return e.Timestamp }

type Resolved struct {
	ID         string    `json:"eventId"`
	IncidentID string    `json:"incidentId"`
	UserID     string    `json:"userId"`
	WorkflowID string    `json:"workflowId"`
	Timestamp  time.Time `json:"occurredAt"`
}

func (e Resolved) EventID() string       { return e.ID }
func (e Resolved) EventType() string     { return EventTypeResolved }
func (e Resolved) AggregateID() string   { return e.IncidentID }
func (e Resolved) OccurredAt() time.Time { return e.Timestamp }

type Reopened struct {
	ID         string    `json:"eventId"`
	IncidentID string    `json:"incidentId"`
	UserID     string    `json:"userId"`
	WorkflowID string    `json:"workflowId"`
	Timestamp  time.Time `json:"occurredAt"`
}

func (e Reopened) EventID() string       { return e.ID }
func (e Reopened) EventType() string     { return EventTypeReopened }
func (e Reopened) AggregateID() string   { return e.IncidentID }
func (e Reopened) OccurredAt() time.Time { return e.Timestamp }

func NewOpenedEvent(inc Incident, at time.Time) event.DomainEvent {
	return Opened{
		ID: uuid.New().String(), IncidentID: inc.ID.String(),
		UserID: inc.UserID.String(), WorkflowID: inc.WorkflowID.String(),
		Kind: string(inc.Kind), Timestamp: at,
	}
}

func NewAcknowledgedEvent(inc Incident, at time.Time) event.DomainEvent {
	return Acknowledged{
		ID: uuid.New().String(), IncidentID: inc.ID.String(),
		UserID: inc.UserID.String(), WorkflowID: inc.WorkflowID.String(),
		Timestamp: at,
	}
}

func NewResolvedEvent(inc Incident, at time.Time) event.DomainEvent {
	return Resolved{
		ID: uuid.New().String(), IncidentID: inc.ID.String(),
		UserID: inc.UserID.String(), WorkflowID: inc.WorkflowID.String(),
		Timestamp: at,
	}
}

func NewReopenedEvent(inc Incident, at time.Time) event.DomainEvent {
	return Reopened{
		ID: uuid.New().String(), IncidentID: inc.ID.String(),
		UserID: inc.UserID.String(), WorkflowID: inc.WorkflowID.String(),
		Timestamp: at,
	}
}
