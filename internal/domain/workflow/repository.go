package workflow

import (
	"context"
	"time"

	"go-api/internal/domain/paginate"
	"go-api/internal/domain/schedule"

	"github.com/google/uuid"
)

type WriteRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// Save inserts the workflow, its calls, rules and static variables.
	Save(ctx context.Context, w *Workflow) error
	// Update replaces the definition.
	Update(ctx context.Context, w *Workflow) error
	Delete(ctx context.Context, id uuid.UUID) error
	// GetByID returns nil, nil when the workflow does not exist.
	GetByID(ctx context.Context, id uuid.UUID) (*Workflow, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Workflow, error)
}

type ReadRepository interface {
	// FindByID returns nil, nil when the workflow does not exist for that user.
	FindByID(ctx context.Context, userID, id uuid.UUID) (*View, error)
	// List returns one page of the user's workflows and the total before pagination.
	List(ctx context.Context, userID uuid.UUID, q paginate.PaginateQuery) ([]ListView, int64, error)
	// ListCalls returns the calls of a workflow ordered by position. Empty when
	// the workflow is missing or belongs to another user.
	ListCalls(ctx context.Context, userID, workflowID uuid.UUID) ([]CallListView, error)
	// FindCall returns nil, nil when the call or its workflow is missing for that user.
	FindCall(ctx context.Context, userID, workflowID, callID uuid.UUID) (*CallView, error)
}

type ListView struct {
	ID   uuid.UUID
	Name string
}

type CallListView struct {
	ID   uuid.UUID
	Name string
}

// CallView is the detail of one call (without extractions and rules for now).
type CallView struct {
	ID        uuid.UUID
	Name      string
	Method    string
	URL       string
	Headers   map[string]string
	Query     map[string]string
	Body      string
	TimeoutMS int
	VerifyTLS bool
	IPVersion string
	ProxyURL  string
}

// View is the read model of a workflow with its current scheduling state, without its calls.
type View struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	ScheduleType    ScheduleType
	IntervalSeconds int
	RunAt           *time.Time
	Enabled         bool
	Status          schedule.Status
	NextRunAt       *time.Time
	LastRunAt       *time.Time
	LastStatus      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
