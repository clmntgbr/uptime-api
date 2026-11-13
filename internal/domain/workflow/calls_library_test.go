package workflow

import (
	"errors"
	"testing"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
)

func newLibraryTestWorkflow(t *testing.T, names ...string) *Workflow {
	t.Helper()
	calls := make([]domaincall.Definition, 0, len(names))
	for _, name := range names {
		calls = append(calls, domaincall.Definition{Name: name, Method: "GET", URL: "https://example.com/" + name})
	}
	w, err := New(uuid.New(), Definition{Name: "wf", Interval: MinInterval, Enabled: true, Calls: calls}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func callNames(w *Workflow) []string {
	out := make([]string, 0, len(w.Calls))
	for _, c := range w.Calls {
		out = append(out, c.Name)
	}
	return out
}

func TestInsertCall_InsertsAtPositionAndRenumbers(t *testing.T) {
	w := newLibraryTestWorkflow(t, "a", "b")
	created, err := w.InsertCall(domaincall.Definition{Name: "x", Method: "GET", URL: "https://example.com/x"}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.Position != 2 {
		t.Fatalf("position: got %d want 2", created.Position)
	}
	if got := callNames(w); len(got) != 3 || got[0] != "a" || got[1] != "x" || got[2] != "b" {
		t.Fatalf("order: got %v", got)
	}
	for i, c := range w.Calls {
		if c.Position != i+1 {
			t.Fatalf("call %q position %d want %d", c.Name, c.Position, i+1)
		}
	}
	if len(w.PullEvents()) == 0 {
		t.Fatal("insert must record a workflow mutation event")
	}
}

func TestInsertCall_AppendsAtEndAndAtStart(t *testing.T) {
	w := newLibraryTestWorkflow(t, "a")
	if _, err := w.InsertCall(domaincall.Definition{Name: "end", Method: "GET", URL: "https://example.com"}, 2, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.InsertCall(domaincall.Definition{Name: "start", Method: "GET", URL: "https://example.com"}, 1, nil); err != nil {
		t.Fatal(err)
	}
	if got := callNames(w); got[0] != "start" || got[1] != "a" || got[2] != "end" {
		t.Fatalf("order: got %v", got)
	}
}

func TestInsertCall_RejectsPositionOutOfRange(t *testing.T) {
	for _, pos := range []int{0, -1, 4} {
		w := newLibraryTestWorkflow(t, "a", "b")
		_, err := w.InsertCall(domaincall.Definition{Name: "x", Method: "GET", URL: "https://example.com"}, pos, nil)
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("position %d: expected ValidationError, got %v", pos, err)
		}
		if len(w.Calls) != 2 {
			t.Fatalf("position %d: workflow must not change", pos)
		}
	}
}

func TestInsertCall_RejectsWhenWorkflowFull(t *testing.T) {
	names := make([]string, MaxCalls)
	for i := range names {
		names[i] = "c"
	}
	w := newLibraryTestWorkflow(t, names...)
	_, err := w.InsertCall(domaincall.Definition{Name: "x", Method: "GET", URL: "https://example.com"}, 1, nil)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid workflow error, got %v", err)
	}
}

func TestInsertCall_RejectsInvalidDefinition(t *testing.T) {
	w := newLibraryTestWorkflow(t, "a")
	_, err := w.InsertCall(domaincall.Definition{Name: "", Method: "GET", URL: "https://example.com"}, 1, nil)
	if err == nil {
		t.Fatal("expected validation error on empty name")
	}
	if len(w.Calls) != 1 {
		t.Fatal("workflow must not change on invalid definition")
	}
}

func TestInsertCall_RejectsVariableProducedLater(t *testing.T) {
	w := newLibraryTestWorkflow(t, "a")
	// The inserted call consumes {{token}}, which no call produces.
	_, err := w.InsertCall(domaincall.Definition{Name: "x", Method: "GET", URL: "https://example.com/{{token}}"}, 1, nil)
	if err == nil {
		t.Fatal("expected unknown variable error")
	}
	if len(w.Calls) != 1 {
		t.Fatal("workflow must not change when variables do not resolve")
	}
}

func TestCallDefinition_CopiesFieldsWithoutIDs(t *testing.T) {
	w := newLibraryTestWorkflow(t, "a")
	def, err := w.CallDefinition(w.Calls[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "a" || def.URL != "https://example.com/a" || def.Method != "GET" {
		t.Fatalf("unexpected definition %+v", def)
	}
	if _, err := w.CallDefinition(uuid.New()); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("expected ErrCallNotFound, got %v", err)
	}
}
