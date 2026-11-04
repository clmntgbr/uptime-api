package incident

import (
	"context"
	"time"

	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type PolicyRepository interface {
	Get(ctx context.Context, workflowID uuid.UUID, kind Kind) (*Policy, error)
	ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) ([]Policy, error)
	Upsert(ctx context.Context, p Policy) error
}

type SignalStateRepository interface {
	// LockGetOrCreate locks the row FOR UPDATE, inserting an empty row if missing.
	LockGetOrCreate(ctx context.Context, workflowID uuid.UUID, fingerprint string) (SignalState, error)
	Save(ctx context.Context, s SignalState) error
}

type IncidentRepository interface {
	FindActive(ctx context.Context, workflowID uuid.UUID, fingerprint string) (*Incident, error)
	FindLatestResolved(ctx context.Context, workflowID uuid.UUID, fingerprint string) (*Incident, error)
	Insert(ctx context.Context, inc Incident) error
	Update(ctx context.Context, inc Incident) error
	GetByID(ctx context.Context, userID, id uuid.UUID) (*Incident, error)
	ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID, states []State, severities []Severity, q paginate.PaginateQuery) ([]Incident, int64, error)
	ListOpenForReminders(ctx context.Context, now time.Time) ([]Incident, error)
}

type EventRepository interface {
	Append(ctx context.Context, e Event) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]Event, error)
}
