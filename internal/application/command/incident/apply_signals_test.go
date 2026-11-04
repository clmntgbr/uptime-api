package incident

import (
	"context"
	"testing"
	"time"

	"go-api/internal/domain/event"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/paginate"
	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

type memPolicies struct {
	byKey map[string]domainincident.Policy
}

func (m *memPolicies) Get(_ context.Context, workflowID uuid.UUID, kind domainincident.Kind) (*domainincident.Policy, error) {
	p, ok := m.byKey[workflowID.String()+string(kind)]
	if !ok {
		return nil, nil
	}
	return &p, nil
}
func (m *memPolicies) ListByWorkflow(context.Context, uuid.UUID, uuid.UUID) ([]domainincident.Policy, error) {
	return nil, nil
}
func (m *memPolicies) Upsert(context.Context, domainincident.Policy) error { return nil }

type memStates struct {
	rows map[string]domainincident.SignalState
}

func (m *memStates) LockGetOrCreate(_ context.Context, workflowID uuid.UUID, fp string) (domainincident.SignalState, error) {
	key := workflowID.String() + fp
	if s, ok := m.rows[key]; ok {
		return s, nil
	}
	s := domainincident.SignalState{WorkflowID: workflowID, Fingerprint: fp}
	m.rows[key] = s
	return s, nil
}
func (m *memStates) Save(_ context.Context, s domainincident.SignalState) error {
	m.rows[s.WorkflowID.String()+s.Fingerprint] = s
	return nil
}

type memIncidents struct {
	active   map[string]*domainincident.Incident
	resolved map[string]*domainincident.Incident
	byID     map[uuid.UUID]*domainincident.Incident
}

func (m *memIncidents) FindActive(_ context.Context, workflowID uuid.UUID, fp string) (*domainincident.Incident, error) {
	return m.active[workflowID.String()+fp], nil
}
func (m *memIncidents) FindLatestResolved(_ context.Context, workflowID uuid.UUID, fp string) (*domainincident.Incident, error) {
	return m.resolved[workflowID.String()+fp], nil
}
func (m *memIncidents) Insert(_ context.Context, inc domainincident.Incident) error {
	cp := inc
	m.active[inc.WorkflowID.String()+inc.Fingerprint] = &cp
	m.byID[inc.ID] = &cp
	return nil
}
func (m *memIncidents) Update(_ context.Context, inc domainincident.Incident) error {
	cp := inc
	m.byID[inc.ID] = &cp
	key := inc.WorkflowID.String() + inc.Fingerprint
	if domainincident.IsActive(inc.State) {
		m.active[key] = &cp
		delete(m.resolved, key)
	} else {
		delete(m.active, key)
		m.resolved[key] = &cp
	}
	return nil
}
func (m *memIncidents) GetByID(_ context.Context, _ uuid.UUID, id uuid.UUID) (*domainincident.Incident, error) {
	return m.byID[id], nil
}
func (m *memIncidents) GetByIDForUpdate(ctx context.Context, userID, id uuid.UUID) (*domainincident.Incident, error) {
	return m.GetByID(ctx, userID, id)
}
func (m *memIncidents) ClaimReminder(_ context.Context, id uuid.UUID, expectedCount int, now time.Time) (bool, error) {
	inc := m.byID[id]
	if inc == nil || inc.State != domainincident.StateOpen || inc.ReminderCount != expectedCount {
		return false, nil
	}
	inc.ReminderCount++
	t := now
	inc.LastReminderAt = &t
	return true, nil
}
func (m *memIncidents) ListByWorkflow(context.Context, uuid.UUID, uuid.UUID, []domainincident.State, []domainincident.Severity, paginate.PaginateQuery) ([]domainincident.Incident, int64, error) {
	return nil, 0, nil
}
func (m *memIncidents) ListOpenForReminders(context.Context, time.Time) ([]domainincident.Incident, error) {
	return nil, nil
}

type memEvents struct{ n int }

func (m *memEvents) Append(context.Context, domainincident.Event) error { m.n++; return nil }
func (m *memEvents) ListByIncident(context.Context, uuid.UUID) ([]domainincident.Event, error) {
	return nil, nil
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

func newApplyHandler() (*ApplySignalsHandler, *memStates, *memIncidents) {
	states := &memStates{rows: map[string]domainincident.SignalState{}}
	incs := &memIncidents{
		active: map[string]*domainincident.Incident{},
		resolved: map[string]*domainincident.Incident{},
		byID: map[uuid.UUID]*domainincident.Incident{},
	}
	h := NewApplySignalsHandler(&memPolicies{byKey: map[string]domainincident.Policy{}}, states, incs, &memEvents{}, &memOutbox{})
	h.clock = func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) }
	return h, states, incs
}

func TestApplySignals_IdempotentSameRun(t *testing.T) {
	h, states, _ := newApplyHandler()
	user, wf, run := uuid.New(), uuid.New(), uuid.New()
	call := uuid.New()
	at := time.Date(2026, 11, 4, 11, 0, 0, 0, time.UTC)
	sig := domainincident.Signal{
		Kind: domainincident.KindCallFailed, Failing: true,
		CallID: &call, Subject: domainincident.CallSubject(call),
		RunID: &run, ObservedAt: at,
	}
	cmd := ApplySignalsCommand{UserID: user, WorkflowID: wf, Signals: []domainincident.Signal{sig}}
	if _, err := h.Handle(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Handle(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	fp := domainincident.Fingerprint(wf, domainincident.KindCallFailed, domainincident.CallSubject(call))
	st := states.rows[wf.String()+fp]
	if st.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1 failure after double apply, got %d", st.ConsecutiveFailures)
	}
}

func TestApplySignals_OlderSignalIgnored(t *testing.T) {
	h, states, _ := newApplyHandler()
	user, wf := uuid.New(), uuid.New()
	call := uuid.New()
	run1, run2 := uuid.New(), uuid.New()
	t1 := time.Date(2026, 11, 4, 11, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	base := ApplySignalsCommand{UserID: user, WorkflowID: wf}
	newer := domainincident.Signal{
		Kind: domainincident.KindCallFailed, Failing: true,
		CallID: &call, Subject: domainincident.CallSubject(call),
		RunID: &run2, ObservedAt: t2,
	}
	older := newer
	older.RunID = &run1
	older.ObservedAt = t1
	base.Signals = []domainincident.Signal{newer}
	if _, err := h.Handle(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	base.Signals = []domainincident.Signal{older}
	if _, err := h.Handle(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	fp := domainincident.Fingerprint(wf, domainincident.KindCallFailed, domainincident.CallSubject(call))
	if states.rows[wf.String()+fp].ConsecutiveFailures != 1 {
		t.Fatal("older signal must not change counters")
	}
}

func TestApplySignals_OpenAfterThree(t *testing.T) {
	h, _, incs := newApplyHandler()
	user, wf := uuid.New(), uuid.New()
	call := uuid.New()
	at := time.Date(2026, 11, 4, 11, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		run := uuid.New()
		sig := domainincident.Signal{
			Kind: domainincident.KindCallFailed, Failing: true,
			CallID: &call, Subject: domainincident.CallSubject(call),
			RunID: &run, ObservedAt: at.Add(time.Duration(i) * time.Minute),
		}
		if _, err := h.Handle(context.Background(), ApplySignalsCommand{
			UserID: user, WorkflowID: wf, Signals: []domainincident.Signal{sig},
		}); err != nil {
			t.Fatal(err)
		}
	}
	fp := domainincident.Fingerprint(wf, domainincident.KindCallFailed, domainincident.CallSubject(call))
	if incs.active[wf.String()+fp] == nil {
		t.Fatal("expected open incident after 3 failures")
	}
}
