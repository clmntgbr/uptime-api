package schedule

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go-api/internal/contract"
	domaincall "go-api/internal/domain/call"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type capturePublisher struct {
	job contract.Job
}

func (p *capturePublisher) PublishJob(_ context.Context, job contract.Job) error {
	p.job = job
	return nil
}

type captureSealer struct{}

func (captureSealer) Seal(map[string]string) (*contract.SealedSecrets, error) {
	return &contract.SealedSecrets{}, nil
}

func TestDispatchUsesEstimatedBudgetForLeaseAndJob(t *testing.T) {
	now := time.Date(2026, 11, 19, 17, 0, 0, 0, time.UTC)
	userID := uuid.New()
	w := &domainworkflow.Workflow{
		ID:           uuid.New(),
		UserID:       userID,
		Name:         "multi",
		Enabled:      true,
		Timeout:      5 * time.Minute, // legacy field must not drive the lease
		ScheduleType: domainworkflow.ScheduleInterval,
		Interval:     time.Minute,
		Calls: []domaincall.Call{
			{ID: uuid.New(), Name: "a", Method: "GET", URL: "https://a.example", Timeout: time.Minute, Position: 1},
			{ID: uuid.New(), Name: "b", Method: "GET", URL: "https://b.example", Timeout: time.Minute, Position: 2},
			{ID: uuid.New(), Name: "c", Method: "GET", URL: "https://c.example", Timeout: time.Minute, Position: 3},
		},
	}
	budget := domainworkflow.EstimateRunBudget(w.Calls)
	// 3m + 6s + 30s = 3m36s
	wantBudget := 3*time.Minute + 36*time.Second
	if budget != wantBudget {
		t.Fatalf("budget = %v want %v", budget, wantBudget)
	}

	state := domainschedule.New(w.ID, now.Add(-time.Minute))
	states := &memStates{due: []domainschedule.Due{{
		State: state, UserID: userID, Enabled: true,
		ScheduleType: string(domainworkflow.ScheduleInterval), Interval: time.Minute,
	}}}
	workflows := &memWorkflows{byID: map[uuid.UUID]*domainworkflow.Workflow{w.ID: w}}
	publisher := &capturePublisher{}

	h := NewDispatchDueRunsHandler(
		states, workflows, noopGlobals{}, noopRuns{}, noopProcessed{},
		&memOutbox{}, captureSealer{}, publisher, fixedClock{t: now},
		DispatchConfig{BatchSize: 10, LeaseMargin: time.Minute, MaxJitter: 0},
		slog.Default(),
	)

	stats, err := h.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Dispatched != 1 {
		t.Fatalf("dispatched = %d want 1", stats.Dispatched)
	}
	if publisher.job.TimeoutMS != int(wantBudget/time.Millisecond) {
		t.Fatalf("job TimeoutMS = %d want %d", publisher.job.TimeoutMS, wantBudget/time.Millisecond)
	}
	wantDeadline := now.Add(wantBudget)
	if !publisher.job.Deadline.Equal(wantDeadline) {
		t.Fatalf("deadline = %v want %v", publisher.job.Deadline, wantDeadline)
	}
	if len(states.saved) == 0 || states.saved[len(states.saved)-1].LeaseUntil == nil {
		t.Fatal("expected leased state")
	}
	leaseUntil := *states.saved[len(states.saved)-1].LeaseUntil
	wantLease := now.Add(wantBudget + time.Minute)
	if !leaseUntil.Equal(wantLease) {
		t.Fatalf("lease until = %v want %v", leaseUntil, wantLease)
	}
}

func TestManualRunUsesEstimatedBudget(t *testing.T) {
	now := time.Date(2026, 11, 19, 17, 0, 0, 0, time.UTC)
	userID := uuid.New()
	w := &domainworkflow.Workflow{
		ID:           uuid.New(),
		UserID:       userID,
		Name:         "manual",
		Enabled:      true,
		Timeout:      5 * time.Minute,
		ScheduleType: domainworkflow.ScheduleInterval,
		Interval:     time.Minute,
		Calls: []domaincall.Call{
			{ID: uuid.New(), Name: "a", Method: "GET", URL: "https://a.example", Timeout: 45 * time.Second, Position: 1},
		},
	}
	budget := domainworkflow.EstimateRunBudget(w.Calls)
	wantBudget := time.Minute + 17*time.Second // 45s + 2s + 30s

	states := &memStates{}
	// Manual path uses GetForUpdate, not LockDue.
	state := domainschedule.New(w.ID, now)
	states.get = &state

	workflows := &memWorkflows{byID: map[uuid.UUID]*domainworkflow.Workflow{w.ID: w}}
	publisher := &capturePublisher{}

	h := NewRunManualHandler(
		states, workflows, noopGlobals{}, noopRuns{}, noopProcessed{},
		&memOutbox{}, captureSealer{}, publisher, fixedClock{t: now},
		time.Minute, slog.Default(),
	)

	runID, err := h.Handle(context.Background(), RunManualCommand{UserID: userID, WorkflowID: w.ID})
	if err != nil {
		t.Fatal(err)
	}
	if runID == uuid.Nil {
		t.Fatal("expected run id")
	}
	if budget != wantBudget {
		t.Fatalf("budget = %v want %v", budget, wantBudget)
	}
	if publisher.job.TimeoutMS != int(wantBudget/time.Millisecond) {
		t.Fatalf("job TimeoutMS = %d want %d", publisher.job.TimeoutMS, wantBudget/time.Millisecond)
	}
	if len(states.saved) == 0 || states.saved[0].LeaseUntil == nil {
		t.Fatal("expected leased state")
	}
	wantLease := now.Add(wantBudget + time.Minute)
	if !states.saved[0].LeaseUntil.Equal(wantLease) {
		t.Fatalf("lease until = %v want %v", *states.saved[0].LeaseUntil, wantLease)
	}
}
