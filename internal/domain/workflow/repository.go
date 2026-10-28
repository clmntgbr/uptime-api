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
	// ListContextStatics returns workflow statics for the context panel.
	// Secret values are not loaded (Value is empty when Secret is true).
	// Empty when the workflow is missing or unowned.
	ListContextStatics(ctx context.Context, userID, workflowID uuid.UUID) ([]ContextStaticView, error)
	// ListContextExtractions returns call-scoped variable definitions ordered by
	// call position (for the context variables panel). Empty when missing/unowned.
	ListContextExtractions(ctx context.Context, userID, workflowID uuid.UUID) ([]ContextExtractionView, error)
}

// ContextStaticView is one static variable for the context panel.
type ContextStaticView struct {
	Name   string
	Value  string // empty when Secret
	Secret bool
}

// Context variable scopes for GET .../context/variables.
const (
	ContextScopeGlobal   = "global"
	ContextScopeStatic   = "static"
	ContextScopeWorkflow = "workflow"

	ContextVarStatusOK      = "ok"
	ContextVarStatusMissing = "missing"

	// ContextValueMaxChars is the display truncation for context variable values.
	ContextValueMaxChars = 200
)

// ContextExtractionView is one call variable definition (no runtime value).
type ContextExtractionView struct {
	Name     string
	Secret   bool
	CallID   uuid.UUID
	CallName string
	Position int
}

// ContextVariableView is one row of GET .../context/variables.
type ContextVariableView struct {
	Scope      string
	Name       string
	Value      *string
	Secret     bool
	Truncated  bool
	ProducedBy *ContextProducerView
	Status     string // ok|missing for workflow scope; empty otherwise
}

// ContextProducerView identifies the call that extracts a workflow variable.
type ContextProducerView struct {
	CallID uuid.UUID
	Name   string
}

type ListView struct {
	ID   uuid.UUID
	Name string
}

// CallRecentRunLimit is how many past executions feed the list sparkline.
const CallRecentRunLimit = 24

// CallRecentRunTimings is the phase breakdown stored on a call run.
type CallRecentRunTimings struct {
	DNS      int64
	Connect  int64
	TLS      int64
	Transfer int64
}

// CallRecentRunView is one bar of the call list sparkline (status + latency).
type CallRecentRunView struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	Status     string
	HTTPStatus *int
	FinishedAt *time.Time
	LatencyMs  *int64
	Timings    *CallRecentRunTimings
}

// CallListView is one row of GET /workflows/:id/calls (definition + recent runs).
type CallListView struct {
	ID             uuid.UUID
	Name           string
	Method         string
	URL            string
	Position       int
	LastStatus     string
	LastHTTPStatus *int
	LastFinishedAt *time.Time
	LastLatencyMs  *int64
	// RecentRuns is oldest→newest (at most CallRecentRunLimit), for the mini timeline.
	RecentRuns []CallRecentRunView
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
