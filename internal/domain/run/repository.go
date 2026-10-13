package run

import (
	"context"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type WriteRepository interface {
	// Save inserts the run with its call and rule runs.
	Save(ctx context.Context, r *Run) error
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
	StartedAt          *time.Time
	FinishedAt         *time.Time
}
