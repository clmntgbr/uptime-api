package workflow

import (
	"testing"

	domaincall "go-api/internal/domain/call"

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
