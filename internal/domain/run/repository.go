package run

import (
	"context"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type WriteRepository interface {
	// Save upserts the run and its call/rule runs (final result or full insert).
	Save(ctx context.Context, r *Run) error
	// EnsureRunning creates the run as status=running when missing.
	// created is true only when a new row was inserted.
	EnsureRunning(ctx context.Context, r *Run) (created bool, err error)
	// UpsertCallRun inserts or replaces one call_run by (run_id, position).
	UpsertCallRun(ctx context.Context, run *Run, call CallRun) error
}

// Retention expires old results.
type Retention interface {
	// DeleteRunsBefore deletes the runs started before cutoff, with their call and rule runs.
	DeleteRunsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type ReadRepository interface {
	ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID, q paginate.PaginateQuery) ([]View, int64, error)
	// FindByID returns nil, nil when the run does not exist for that user.
	FindByID(ctx context.Context, userID, runID uuid.UUID) (*DetailView, error)
	// LatestResponseKeysForCall returns JSON paths from the most recent call_run
	// that recorded keys for that call. Empty slice when none yet.
	LatestResponseKeysForCall(ctx context.Context, userID, workflowID, callID uuid.UUID) ([]string, error)
}

type View struct {
	ID         uuid.UUID
	WorkflowID uuid.UUID
	Trigger    string
	Status     Status
	Reason     string
	StartedAt  time.Time
	FinishedAt time.Time
	DurationMS int
	Counts     Counts
}

type DetailView struct {
	View
	Variables map[string]string
	Calls     []CallRunView
}

type CallRunView struct {
	ID                 uuid.UUID
	CallID             *uuid.UUID
	Name               string
	Position           int
	Status             CallStatus
	Reason             string
	HTTPStatus         int
	Request            contract.RequestInfo
	Response           contract.ResponseInfo
	Timings            contract.Timings
	Network            contract.NetworkInfo
	TLS                *contract.TLSInfo
	Rules              []RuleRun
	ExtractedVariables map[string]string
	UsedVariables      []string
	Error              *contract.CallError
	BodyRef            string
	BodyHash           string
	BodyEncoding       string
	ResponseKeys       []string
	StartedAt          *time.Time
	FinishedAt         *time.Time
}
