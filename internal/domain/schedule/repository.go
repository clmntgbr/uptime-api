package schedule

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Due is a workflow whose next_run_at has passed, locked for the current transaction.
type Due struct {
	State    State
	UserID   uuid.UUID
	Interval time.Duration
}

type Repository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// LockDue locks (FOR UPDATE SKIP LOCKED) enabled workflows due at now.
	LockDue(ctx context.Context, now time.Time, limit int) ([]Due, error)
	// GetForUpdate returns nil, nil when the state does not exist.
	GetForUpdate(ctx context.Context, workflowID uuid.UUID) (*State, error)
	// Save inserts the state or replaces it.
	Save(ctx context.Context, state State) error
}
