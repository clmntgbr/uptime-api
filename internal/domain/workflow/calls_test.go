package workflow

import (
	"errors"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	domainrule "go-api/internal/domain/rule"

	"github.com/google/uuid"
)

func TestReplaceCall_KeepsExtractionsWhenOmitted(t *testing.T) {
	userID := uuid.New()
	extID := uuid.New()
	w, err := New(userID, Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{{
			Name:   "login",
			Method: "POST",
			URL:    "https://example.com/login",
			Extractions: []domaincall.Extraction{{
				ID: extID, Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token",
			}},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	replaced, err := w.ReplaceCall(w.Calls[0].ID, domaincall.Definition{
		Name:   "login",
		Method: "POST",
		URL:    "https://example.com/login-v2",
		// Extractions omitted — call PUT payloads usually leave them empty.
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced.Extractions) != 1 || replaced.Extractions[0].ID != extID || replaced.Extractions[0].Name != "token" {
		t.Fatalf("call variables must survive a call save without extractions, got %+v", replaced.Extractions)
	}
	if replaced.URL != "https://example.com/login-v2" {
		t.Fatalf("url must still update, got %s", replaced.URL)
	}
}

func TestReplaceCall_IgnoresDefinitionExtractions(t *testing.T) {
	userID := uuid.New()
	extID := uuid.New()
	w, err := New(userID, Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{{
			Name:   "login",
			Method: "POST",
			URL:    "https://example.com/login",
			Extractions: []domaincall.Extraction{{
				ID: extID, Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token",
			}},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	replaced, err := w.ReplaceCall(w.Calls[0].ID, domaincall.Definition{
		Name:   "login",
		Method: "POST",
		URL:    "https://example.com/login",
		Extractions: []domaincall.Extraction{{
			Name: "other", Source: domaincall.ExtractJSON, Expression: "$.other",
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced.Extractions) != 1 || replaced.Extractions[0].ID != extID ||
		replaced.Extractions[0].Name != "token" || replaced.Extractions[0].Expression != "$.token" {
		t.Fatalf("ReplaceCall must keep existing call variables, got %+v", replaced.Extractions)
	}
}

func TestDuplicateCall_CopiesFieldsAndRulesKeepsTemplates(t *testing.T) {
	userID := uuid.New()
	verify := true
	w, err := New(userID, Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{
			{
				Name:   "login",
				Method: "POST",
				URL:    "https://example.com/login",
				Extractions: []domaincall.Extraction{{
					Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token",
				}},
			},
			{
				Name:      "Orders call",
				Method:    "GET",
				URL:       "https://example.com/orders?access={{token}}",
				Headers:   map[string]string{"Authorization": "Bearer {{token}}"},
				Query:     map[string]string{"page": "1"},
				Body:      `{"token":"{{token}}"}`,
				Timeout:   5 * time.Second,
				VerifyTLS: &verify,
				IPVersion: domaincall.IPv4,
				ProxyURL:  "https://proxy.example.com:8080",
				Extractions: []domaincall.Extraction{{
					Name: "orderId", Source: domaincall.ExtractJSON, Expression: "$.id",
				}},
				Rules: []domainrule.Rule{{
					Subject: domainrule.SubjectStatus, Operator: domainrule.OpIs, Value: "200",
				}},
			},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srcID := w.Calls[1].ID

	copied, err := w.DuplicateCall(srcID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == srcID || copied.Position != 3 || copied.Name != "Orders call" ||
		copied.Method != "GET" ||
		copied.URL != "https://example.com/orders?access={{token}}" ||
		copied.Headers["Authorization"] != "Bearer {{token}}" ||
		copied.Query["page"] != "1" || copied.Body != `{"token":"{{token}}"}` ||
		copied.Timeout != 5*time.Second || !copied.VerifyTLS ||
		copied.IPVersion != domaincall.IPv4 ||
		copied.ProxyURL != "https://proxy.example.com:8080" {
		t.Fatalf("unexpected copied call %+v", copied)
	}
	if len(copied.Rules) != 1 || copied.Rules[0].ID == uuid.Nil ||
		copied.Rules[0].Subject != domainrule.SubjectStatus || copied.Rules[0].Value != "200" {
		t.Fatalf("rules not copied: %+v", copied.Rules)
	}
	if len(copied.Extractions) != 0 {
		t.Fatalf("call variables must not be copied, got %+v", copied.Extractions)
	}
	if len(w.Calls[1].Extractions) != 1 || w.Calls[1].Extractions[0].Name != "orderId" {
		t.Fatalf("source extractions must stay untouched: %+v", w.Calls[1].Extractions)
	}
}

func TestReorderCalls_Success(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{Name: "a", Method: "GET", URL: "https://example.com/a"},
			{Name: "b", Method: "GET", URL: "https://example.com/b"},
			{Name: "c", Method: "GET", URL: "https://example.com/c"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := w.Calls[0].ID, w.Calls[1].ID, w.Calls[2].ID

	if err := w.ReorderCalls([]uuid.UUID{c, a, b}, nil); err != nil {
		t.Fatal(err)
	}
	if w.Calls[0].ID != c || w.Calls[0].Position != 1 ||
		w.Calls[1].ID != a || w.Calls[1].Position != 2 ||
		w.Calls[2].ID != b || w.Calls[2].Position != 3 {
		t.Fatalf("unexpected order %+v", w.Calls)
	}
}

func TestReorderCalls_RejectsProducerAfterConsumer(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{
				Name: "login", Method: "POST", URL: "https://example.com/login",
				Extractions: []domaincall.Extraction{{
					Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token",
				}},
			},
			{
				Name: "orders", Method: "GET", URL: "https://example.com/orders?t={{token}}",
			},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	login, orders := w.Calls[0].ID, w.Calls[1].ID

	err = w.ReorderCalls([]uuid.UUID{orders, login}, nil)
	var verr *ValidationError
	if err == nil || !errors.As(err, &verr) {
		t.Fatalf("got %v want ValidationError", err)
	}
}

func TestReorderCalls_IncompleteList(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{Name: "a", Method: "GET", URL: "https://example.com/a"},
			{Name: "b", Method: "GET", URL: "https://example.com/b"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ReorderCalls([]uuid.UUID{w.Calls[0].ID}, nil); err != ErrInvalidCallOrder {
		t.Fatalf("got %v want ErrInvalidCallOrder", err)
	}
}

func TestReorderCalls_UnknownID(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{Name: "a", Method: "GET", URL: "https://example.com/a"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ReorderCalls([]uuid.UUID{uuid.New()}, nil); err != ErrInvalidCallOrder {
		t.Fatalf("got %v want ErrInvalidCallOrder", err)
	}
}

func TestReorderCalls_DuplicateID(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{Name: "a", Method: "GET", URL: "https://example.com/a"},
			{Name: "b", Method: "GET", URL: "https://example.com/b"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := w.Calls[0].ID
	if err := w.ReorderCalls([]uuid.UUID{id, id}, nil); err != ErrInvalidCallOrder {
		t.Fatalf("got %v want ErrInvalidCallOrder", err)
	}
}

func TestDuplicateCall_NotFound(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{{Name: "a", Method: "GET", URL: "https://example.com"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.DuplicateCall(uuid.New(), nil); err != ErrCallNotFound {
		t.Fatalf("got %v want ErrCallNotFound", err)
	}
}

func TestAddCall_AllowsDuplicateNames(t *testing.T) {
	userID := uuid.New()
	w, err := New(userID, Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{{
			Name: "Orders call", Method: "GET", URL: "https://example.com/a",
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.AddCall(domaincall.Definition{
		Name: "Orders call", Method: "GET", URL: "https://example.com/b",
	}, nil)
	if err != nil {
		t.Fatalf("duplicate call names must be allowed: %v", err)
	}
	if second.Name != "Orders call" || len(w.Calls) != 2 || w.Calls[0].ID == w.Calls[1].ID {
		t.Fatalf("unexpected calls %+v", w.Calls)
	}
}

func TestNew_RejectsEnabledWithoutCalls(t *testing.T) {
	_, err := New(uuid.New(), Definition{
		Name: "empty", Interval: MinInterval, Enabled: true,
	}, nil)
	if err != ErrNoCalls {
		t.Fatalf("got %v want ErrNoCalls", err)
	}
}

func TestRemoveCall_DisablesWhenEmpty(t *testing.T) {
	w, err := New(uuid.New(), Definition{
		Name: "wf", Interval: MinInterval, Enabled: true,
		Calls: []domaincall.Definition{
			{Name: "only", Method: "GET", URL: "https://example.com"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RemoveCall(w.Calls[0].ID, nil); err != nil {
		t.Fatal(err)
	}
	if w.Enabled || len(w.Calls) != 0 {
		t.Fatalf("expected disabled empty workflow, got enabled=%v calls=%d", w.Enabled, len(w.Calls))
	}
}

func TestApply_PreservesExtractionIDsByCallName(t *testing.T) {
	userID := uuid.New()
	extID := uuid.New()
	w, err := New(userID, Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{{
			Name:   "login",
			Method: "GET",
			URL:    "https://example.com/a",
			Extractions: []domaincall.Extraction{{
				ID: extID, Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token",
			}},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.apply(Definition{
		Name:     "wf",
		Interval: MinInterval,
		Enabled:  true,
		Calls: []domaincall.Definition{{
			Name:   "login",
			Method: "GET",
			URL:    "https://example.com/b",
			Extractions: []domaincall.Extraction{{
				Name: "token", Source: domaincall.ExtractJSON, Expression: "$.new",
			}},
		}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if w.Calls[0].Extractions[0].ID != extID {
		t.Fatalf("extraction id must be preserved across apply, got %+v", w.Calls[0].Extractions)
	}
}
