package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func loginThenOrders() Definition {
	return Definition{
		Name:     "Checkout",
		Interval: 5 * time.Minute,
		Enabled:  true,
		Statics:  []StaticVariable{{Name: "base_url", Value: "https://api.example.com"}},
		Calls: []CallDefinition{
			{
				Name:        "login",
				Method:      "POST",
				URL:         "{{base_url}}/login",
				Body:        `{"password":"{{password}}"}`,
				Extractions: []Extraction{{Name: "token", Source: ExtractJSON, Expression: "$.access_token"}},
			},
			{
				Name:    "orders",
				URL:     "{{base_url}}/orders",
				Headers: map[string]string{"Authorization": "Bearer {{token}}"},
			},
		},
	}
}

func TestNew_ValidWorkflow(t *testing.T) {
	w, err := New(uuid.New(), loginThenOrders(), []string{"password"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Calls) != 2 || w.Calls[0].Position != 1 || w.Calls[1].Position != 2 {
		t.Fatalf("unexpected calls %+v", w.Calls)
	}
	if w.Calls[1].Method != "GET" || !w.Calls[1].VerifyTLS || w.Calls[1].Timeout != DefaultCallTimeout {
		t.Fatalf("defaults not applied: %+v", w.Calls[1])
	}
	events := w.PullEvents()
	if len(events) != 1 || events[0].EventType() != EventTypeWorkflowCreated {
		t.Fatalf("unexpected events %v", events)
	}
}

func TestNew_VariableProducedLater(t *testing.T) {
	def := loginThenOrders()
	def.Calls[0], def.Calls[1] = def.Calls[1], def.Calls[0]

	_, err := New(uuid.New(), def, []string{"password"})
	var verr *ValidationError
	if !errors.As(err, &verr) || !errors.Is(err, ErrVarOutOfOrder) {
		t.Fatalf("expected out of order error, got %v", err)
	}
	if verr.Call != "orders" || verr.Variable != "token" {
		t.Fatalf("error should name the call and variable: %+v", verr)
	}
}

func TestNew_UnknownVariable(t *testing.T) {
	_, err := New(uuid.New(), loginThenOrders(), nil)
	if !errors.Is(err, ErrUnknownVar) {
		t.Fatalf("expected unknown variable, got %v", err)
	}
}

func TestNew_NameConflict(t *testing.T) {
	_, err := New(uuid.New(), loginThenOrders(), []string{"password", "base_url"})
	if !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected name conflict, got %v", err)
	}

	def := loginThenOrders()
	def.Calls[1].Extractions = []Extraction{{Name: "token", Source: ExtractStatus}}
	_, err = New(uuid.New(), def, []string{"password"})
	if !errors.Is(err, ErrNameConflict) {
		t.Fatalf("expected conflict between call variables, got %v", err)
	}
}

func TestNew_Builtins(t *testing.T) {
	def := loginThenOrders()
	def.Calls[1].Headers["X-Request-Id"] = "{{uuid}}-{{run_id}}-{{timestamp}}"
	if _, err := New(uuid.New(), def, []string{"password"}); err != nil {
		t.Fatalf("builtins must be available: %v", err)
	}
}

func TestNew_IntervalAndCallLimits(t *testing.T) {
	def := loginThenOrders()
	def.Interval = 90 * time.Second
	if _, err := New(uuid.New(), def, []string{"password"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("interval must be whole minutes, got %v", err)
	}
	def.Interval = 30 * time.Second
	if _, err := New(uuid.New(), def, []string{"password"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("interval below 1 minute must fail, got %v", err)
	}

	def = loginThenOrders()
	for i := len(def.Calls); i <= MaxCalls; i++ {
		def.Calls = append(def.Calls, CallDefinition{Name: uuid.NewString(), URL: "https://example.com"})
	}
	if _, err := New(uuid.New(), def, []string{"password"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("more than %d calls must fail, got %v", MaxCalls, err)
	}
}

func TestUpdate_KeepsCallIDsByName(t *testing.T) {
	w, err := New(uuid.New(), loginThenOrders(), []string{"password"})
	if err != nil {
		t.Fatal(err)
	}
	loginID := w.Calls[0].ID
	w.PullEvents()

	def := loginThenOrders()
	def.Calls = append(def.Calls, CallDefinition{Name: "health", URL: "https://example.com/health"})
	changed, err := w.Update(def, []string{"password"})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("schedule did not change")
	}
	if w.Calls[0].ID != loginID {
		t.Fatal("call ID must be kept when the name is kept")
	}

	def.Interval = 10 * time.Minute
	if changed, _ = w.Update(def, []string{"password"}); !changed {
		t.Fatal("interval change must be reported")
	}
}

func TestNextRun_AlignedOnInterval(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 7, 42, 0, time.UTC)
	cases := map[time.Duration]time.Time{
		time.Minute:      time.Date(2026, 10, 13, 10, 8, 0, 0, time.UTC),
		5 * time.Minute:  time.Date(2026, 10, 13, 10, 10, 0, 0, time.UTC),
		15 * time.Minute: time.Date(2026, 10, 13, 10, 15, 0, 0, time.UTC),
		time.Hour:        time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC),
	}
	for interval, want := range cases {
		if got := NextRun(now, interval); !got.Equal(want) {
			t.Errorf("interval %s: got %s want %s", interval, got, want)
		}
	}
	boundary := time.Date(2026, 10, 13, 10, 10, 0, 0, time.UTC)
	if got := NextRun(boundary, 5*time.Minute); !got.Equal(boundary.Add(5 * time.Minute)) {
		t.Errorf("a boundary must move to the next one, got %s", got)
	}
}

func TestState_LeaseLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	s := NewState(uuid.New(), now, time.Minute)
	runID := uuid.New()
	s.Lease(runID, now, 6*time.Minute)

	if !s.LeaseActive(now.Add(5 * time.Minute)) {
		t.Fatal("lease must be active before it expires")
	}
	if s.LeaseActive(now.Add(7 * time.Minute)) {
		t.Fatal("lease must expire")
	}

	other := uuid.New()
	s.Complete(other, "success", now.Add(time.Minute))
	if !s.Running() {
		t.Fatal("a result for another run must not release the lease")
	}

	s.Complete(runID, "failed", now.Add(2*time.Minute))
	if s.Running() || s.ConsecutiveFailures != 1 || s.LastStatus != "failed" {
		t.Fatalf("unexpected state %+v", s)
	}

	s.Lease(uuid.New(), now.Add(3*time.Minute), 6*time.Minute)
	lease, ok := s.ExpireLease()
	if !ok || lease.RunID == uuid.Nil || s.Running() || s.ConsecutiveFailures != 2 {
		t.Fatalf("expired lease must count as an error: %+v", s)
	}
}

func TestParseRule(t *testing.T) {
	cases := []struct {
		text     string
		subject  Subject
		operator Operator
		target   string
		value    string
	}{
		{"status is 200", SubjectStatus, OpIs, "", "200"},
		{"status in 200, 201, 2xx", SubjectStatus, OpIn, "", "200,201,2xx"},
		{"response time exceeds 800 ms", SubjectResponseTime, OpExceeds, "", "800"},
		{"response time exceeds 2 s", SubjectResponseTime, OpExceeds, "", "2000"},
		{"body contains ok", SubjectBody, OpContains, "", "ok"},
		{"body does not contain error", SubjectBody, OpNotContains, "", "error"},
		{"json $.items exists", SubjectJSON, OpExists, "$.items", ""},
		{"json $.status is {{expected}}", SubjectJSON, OpIs, "$.status", "{{expected}}"},
		{"header Content-Type contains json", SubjectHeader, OpContains, "Content-Type", "json"},
		{"size exceeds 64 KB", SubjectSize, OpExceeds, "", "65536"},
		{"certificate expires in less than 14 days", SubjectCertificate, OpExpiresWithin, "", "14"},
		{"certificate chain is invalid", SubjectCertificate, OpChainInvalid, "", ""},
	}
	for _, c := range cases {
		r, err := ParseRule(c.text)
		if err != nil {
			t.Errorf("%q: %v", c.text, err)
			continue
		}
		if r.Subject != c.subject || r.Operator != c.operator || r.Target != c.target || r.Value != c.value {
			t.Errorf("%q: got %+v", c.text, r)
		}
	}

	for _, text := range []string{"status is 999", "latency is high", "response time exceeds fast", "json items exists"} {
		if _, err := ParseRule(text); err == nil {
			t.Errorf("%q must be rejected", text)
		}
	}
}

func TestRuleText_RoundTrip(t *testing.T) {
	for _, text := range []string{
		"status is 200",
		"response time exceeds 800 ms",
		"size exceeds 64 KB",
		"json $.count is below 3",
		"certificate expires in less than 14 days",
	} {
		r, err := ParseRule(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Text(); got != text {
			t.Errorf("got %q want %q", got, text)
		}
	}
}
