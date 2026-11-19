package schedule

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type memStates struct {
	due   []domainschedule.Due
	get   *domainschedule.State
	saved []domainschedule.State
}

func (m *memStates) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *memStates) LockDue(context.Context, time.Time, int) ([]domainschedule.Due, error) {
	return m.due, nil
}
func (m *memStates) Save(_ context.Context, s domainschedule.State) error {
	m.saved = append(m.saved, s)
	if m.get != nil {
		cp := s
		m.get = &cp
	}
	return nil
}
func (m *memStates) GetForUpdate(context.Context, uuid.UUID) (*domainschedule.State, error) {
	return m.get, nil
}

type memWorkflows struct {
	byID    map[uuid.UUID]*domainworkflow.Workflow
	updated []*domainworkflow.Workflow
}

func (m *memWorkflows) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *memWorkflows) Save(context.Context, *domainworkflow.Workflow) error { return nil }
func (m *memWorkflows) Update(_ context.Context, w *domainworkflow.Workflow) error {
	m.updated = append(m.updated, w)
	return nil
}
func (m *memWorkflows) Delete(context.Context, uuid.UUID) error { return nil }
func (m *memWorkflows) GetByID(_ context.Context, id uuid.UUID) (*domainworkflow.Workflow, error) {
	return m.byID[id], nil
}
func (m *memWorkflows) ListByUser(context.Context, uuid.UUID) ([]*domainworkflow.Workflow, error) {
	return nil, nil
}
func (m *memWorkflows) FindWorkflowIDByCall(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

type memOutbox struct{ events []event.DomainEvent }

func (m *memOutbox) StoreEvents(_ context.Context, evs []event.DomainEvent) error {
	m.events = append(m.events, evs...)
	return nil
}
func (m *memOutbox) FetchUnpublished(context.Context, int) ([]port.OutboxMessage, error) {
	return nil, nil
}
func (m *memOutbox) MarkPublished(context.Context, []uuid.UUID) error { return nil }

type noopGlobals struct{}

func (noopGlobals) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (noopGlobals) ListByUser(context.Context, uuid.UUID) ([]variable.Global, error) {
	return nil, nil
}
func (noopGlobals) ListForContext(context.Context, uuid.UUID) ([]variable.Global, error) {
	return nil, nil
}
func (noopGlobals) FindByID(context.Context, uuid.UUID, uuid.UUID) (*variable.Global, error) {
	return nil, nil
}
func (noopGlobals) FindByName(context.Context, uuid.UUID, string) (*variable.Global, error) {
	return nil, nil
}
func (noopGlobals) Names(context.Context, uuid.UUID) ([]string, error) { return nil, nil }
func (noopGlobals) Save(context.Context, *variable.Global) error       { return nil }
func (noopGlobals) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

type noopRuns struct{}

func (noopRuns) Save(context.Context, *domainrun.Run) error { return nil }
func (noopRuns) EnsureRunning(context.Context, *domainrun.Run) (bool, error) {
	return false, nil
}
func (noopRuns) UpsertCallRun(context.Context, *domainrun.Run, domainrun.CallRun) error {
	return nil
}

type noopProcessed struct{}

func (noopProcessed) MarkProcessed(context.Context, string, string) (bool, error) {
	return true, nil
}

type noopSealer struct{}

func (noopSealer) Seal(map[string]string) (*contract.SealedSecrets, error) {
	return nil, nil
}

type noopPublisher struct{ jobs int }

func (p *noopPublisher) PublishJob(context.Context, contract.Job) error {
	p.jobs++
	return nil
}

func TestDispatchDueRuns_EmptyCallsDisablesWithoutRun(t *testing.T) {
	now := time.Date(2026, 11, 19, 16, 50, 0, 0, time.UTC)
	userID := uuid.New()
	w := &domainworkflow.Workflow{
		ID:           uuid.New(),
		UserID:       userID,
		Name:         "empty",
		Enabled:      true,
		ScheduleType: domainworkflow.ScheduleInterval,
		Interval:     time.Minute,
		Calls:        nil,
	}

	state := domainschedule.New(w.ID, now.Add(-time.Minute))
	states := &memStates{due: []domainschedule.Due{{
		State: state, UserID: userID, Enabled: true,
		ScheduleType: string(domainworkflow.ScheduleInterval), Interval: time.Minute,
	}}}
	workflows := &memWorkflows{byID: map[uuid.UUID]*domainworkflow.Workflow{w.ID: w}}
	outbox := &memOutbox{}
	publisher := &noopPublisher{}

	h := NewDispatchDueRunsHandler(
		states, workflows, noopGlobals{}, noopRuns{}, noopProcessed{},
		outbox, noopSealer{}, publisher, fixedClock{t: now},
		DispatchConfig{BatchSize: 10, LeaseMargin: time.Minute, MaxJitter: 0},
		slog.Default(),
	)

	stats, err := h.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Dispatched != 0 || publisher.jobs != 0 {
		t.Fatalf("expected no dispatch, got dispatched=%d published=%d", stats.Dispatched, publisher.jobs)
	}
	if len(workflows.updated) != 1 || workflows.updated[0].Enabled {
		t.Fatalf("workflow must be disabled: %+v", workflows.updated)
	}
	if len(states.saved) != 1 || !states.saved[0].NextRunAt.Equal(domainschedule.Never) {
		t.Fatalf("schedule must be cleared: %+v", states.saved)
	}
	if len(outbox.events) == 0 {
		t.Fatal("expected workflow.updated outbox event")
	}
}
