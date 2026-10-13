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
