package run

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"

	"github.com/google/uuid"
)

type memRuns struct {
	running  *domainrun.Run
	calls    []domainrun.CallRun
	saved    int
	ensured  int
	upserted int
}

func (m *memRuns) Save(context.Context, *domainrun.Run) error { m.saved++; return nil }
func (m *memRuns) EnsureRunning(_ context.Context, r *domainrun.Run) error {
	m.ensured++
	if m.running == nil {
		cp := *r
		m.running = &cp
	}
	return nil
}
func (m *memRuns) UpsertCallRun(_ context.Context, _ *domainrun.Run, call domainrun.CallRun) error {
	m.upserted++
	m.calls = append(m.calls, call)
	return nil
}

type memStates struct{}

func (memStates) WithTransaction(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}
func (memStates) Save(context.Context, domainschedule.State) error          { return nil }
func (memStates) GetForUpdate(context.Context, uuid.UUID) (*domainschedule.State, error) {
	return nil, nil
}
func (memStates) LockDue(context.Context, time.Time, int) ([]domainschedule.Due, error) {
	return nil, nil
}

type memProcessed struct {
	seen map[string]bool
}

func (m *memProcessed) MarkProcessed(_ context.Context, eventID, _ string) (bool, error) {
	if m.seen == nil {
		m.seen = map[string]bool{}
	}
	if m.seen[eventID] {
		return false, nil
	}
	m.seen[eventID] = true
	return true, nil
}

type memOutbox struct{ events []event.DomainEvent }

func (m *memOutbox) StoreEvents(_ context.Context, events []event.DomainEvent) error {
	m.events = append(m.events, events...)
	return nil
}
func (m *memOutbox) FetchUnpublished(context.Context, int) ([]port.OutboxMessage, error) {
	return nil, nil
}
func (m *memOutbox) MarkPublished(context.Context, []uuid.UUID) error { return nil }

func TestRecordCallProgress_CreatesRunningRun(t *testing.T) {
	id := uuid.NewString()
	callID := uuid.NewString()
	progress := contract.CallProgress{
		V: contract.CallProgressVersion, ID: uuid.NewString(), Type: contract.CallProgressType,
		UserID: id, WorkflowID: id, RunID: id, Trigger: contract.TriggerManual,
		Call: contract.CallRun{
			CallID: callID, Name: "login", Order: 1, Status: contract.CallStatusSuccess,
			StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
		},
		EmittedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(progress)

	runs := &memRuns{}
	outbox := &memOutbox{}
	processed := &memProcessed{}
	h := NewRecordCallProgressHandler(memStates{}, runs, processed, outbox)
	if err := h.Handle(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if runs.ensured != 1 || runs.upserted != 1 || runs.running == nil || runs.running.Status != domainrun.StatusRunning {
		t.Fatalf("unexpected state ensured=%d upserted=%d run=%+v", runs.ensured, runs.upserted, runs.running)
	}
	if len(runs.calls) != 1 || runs.calls[0].Name != "login" {
		t.Fatalf("unexpected calls %+v", runs.calls)
	}
	if len(outbox.events) != 1 || outbox.events[0].EventType() != domainrun.EventTypeRunProgressed {
		t.Fatalf("expected run.progressed event, got %+v", outbox.events)
	}

	// Idempotent redelivery
	if err := h.Handle(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if runs.ensured != 1 || runs.upserted != 1 {
		t.Fatalf("redelivery must be a no-op, ensured=%d upserted=%d", runs.ensured, runs.upserted)
	}
}
