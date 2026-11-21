package feed

import (
	"time"

	"github.com/google/uuid"
)

// Event is one append-only feed row.
type Event struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	WorkflowID *uuid.UUID
	RunID      *uuid.UUID
	CallID     *uuid.UUID
	IncidentID *uuid.UUID
	Kind       Kind
	Severity   Severity
	At         time.Time
	Title      string
	Payload    map[string]any
}

// ListQuery filters the chronological feed for one user.
type ListQuery struct {
	UserID     uuid.UUID
	WorkflowID *uuid.UUID
	Kinds      []Kind
	Severities []Severity
	From       *time.Time
	To         *time.Time
	// CursorAt + CursorID page older than this position (DESC). Both nil = first page.
	CursorAt *time.Time
	CursorID *uuid.UUID
	Limit    int
}

const (
	DefaultListLimit = 50
	MaxListLimit     = 100
)

// Normalize clamps limit.
func (q *ListQuery) Normalize() {
	if q.Limit <= 0 {
		q.Limit = DefaultListLimit
	}
	if q.Limit > MaxListLimit {
		q.Limit = MaxListLimit
	}
}
