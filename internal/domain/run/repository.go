package run

import (
	"context"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type WriteRepository interface {
	// Save inserts the run with its call and rule results.
	Save(ctx context.Context, r *Run) error
}

// PartitionMaintainer manages the monthly partitions of the result tables.
type PartitionMaintainer interface {
	EnsurePartitions(ctx context.Context, monthsAhead int) error
	// DropPartitionsBefore drops partitions entirely older than cutoff and returns their names.
	DropPartitionsBefore(ctx context.Context, cutoff time.Time) ([]string, error)
}

// AggregateRepository computes availability and latency summaries kept longer than the details.
type AggregateRepository interface {
	ComputeHourly(ctx context.Context, from, to time.Time) (int64, error)
	ComputeDaily(ctx context.Context, from, to time.Time) (int64, error)
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
	Calls     []CallResultView
}

type CallResultView struct {
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
	Rules              []RuleResult
	ExtractedVariables map[string]string
	UsedVariables      []string
	Error              *contract.CallError
	BodyRef            string
	StartedAt          *time.Time
	FinishedAt         *time.Time
}
