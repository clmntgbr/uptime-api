package workflow

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type WriteRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// Save inserts the workflow, its calls, rules, static variables and initial state.
	Save(ctx context.Context, w *Workflow, state State) error
	// Update replaces the definition; nextRunAt is applied when not nil.
	Update(ctx context.Context, w *Workflow, nextRunAt *time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	// GetByID returns nil, nil when the workflow does not exist.
	GetByID(ctx context.Context, id uuid.UUID) (*Workflow, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Workflow, error)
}

// Due is a workflow whose next_run_at has passed, locked for the current transaction.
type Due struct {
	State    State
	UserID   uuid.UUID
	Interval time.Duration
}

type StateRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// LockDue locks (FOR UPDATE SKIP LOCKED) enabled workflows due at now.
	LockDue(ctx context.Context, now time.Time, limit int) ([]Due, error)
	// GetForUpdate returns nil, nil when the state does not exist.
	GetForUpdate(ctx context.Context, workflowID uuid.UUID) (*State, error)
	Save(ctx context.Context, state State) error
}

type ReadRepository interface {
	// FindByID returns nil, nil when the workflow does not exist for that user.
	FindByID(ctx context.Context, userID, id uuid.UUID) (*View, error)
}

// View is the read model of a workflow with its current scheduling state.
type View struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	IntervalSeconds int
	Enabled         bool
	Calls           []CallView
	StaticNames     []string
	Status          ScheduleStatus
	NextRunAt       *time.Time
	LastRunAt       *time.Time
	LastStatus      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CallView struct {
	ID        uuid.UUID
	Position  int
	Name      string
	Method    string
	URL       string
	RuleCount int
}
