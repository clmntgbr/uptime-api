package library

import (
	"errors"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
)

func validDefinition() domaincall.Definition {
	return domaincall.Definition{Name: "Health", Method: "get", URL: "https://example.com/health"}
}

func TestNew_NormalizesDefinition(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	item, err := New(userID, validDefinition(), now)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == uuid.Nil || item.UserID != userID {
		t.Fatalf("identity not set: %+v", item)
	}
	if item.Definition.Method != "GET" {
		t.Fatalf("method must be normalized, got %q", item.Definition.Method)
	}
	if item.Definition.Timeout != domaincall.DefaultTimeout {
		t.Fatalf("default timeout not applied: %s", item.Definition.Timeout)
	}
	if item.CreatedAt != now || item.UpdatedAt != now {
		t.Fatal("timestamps must equal now on creation")
	}
}

func TestNew_RejectsInvalidDefinition(t *testing.T) {
	def := validDefinition()
	def.URL = ""
	_, err := New(uuid.New(), def, time.Now())
	if !errors.Is(err, domaincall.ErrInvalid) {
		t.Fatalf("expected call validation error, got %v", err)
	}
}

func TestNormalize_ReturnsNormalizedDefinition(t *testing.T) {
	def := validDefinition()
	def.Rules = nil
	normalized, err := Normalize(def)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Name != "Health" || normalized.URL != def.URL {
		t.Fatalf("unexpected normalized definition %+v", normalized)
	}
}

func TestReplace_KeepsIdentityAndMovesUpdatedAt(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	item, err := New(uuid.New(), validDefinition(), created)
	if err != nil {
		t.Fatal(err)
	}
	id := item.ID

	later := created.Add(time.Hour)
	def := validDefinition()
	def.Name = "Renamed"
	if err := item.Replace(def, later); err != nil {
		t.Fatal(err)
	}
	if item.ID != id || item.Definition.Name != "Renamed" {
		t.Fatalf("replace must keep id and apply definition: %+v", item)
	}
	if !item.CreatedAt.Equal(created) || !item.UpdatedAt.Equal(later) {
		t.Fatalf("timestamps: created %s updated %s", item.CreatedAt, item.UpdatedAt)
	}
}

func TestReplace_InvalidDefinitionKeepsItem(t *testing.T) {
	item, err := New(uuid.New(), validDefinition(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	bad := validDefinition()
	bad.Method = "TRACE"
	if err := item.Replace(bad, time.Now()); err == nil {
		t.Fatal("expected validation error")
	}
	if item.Definition.Method != "GET" {
		t.Fatal("item must not change on invalid replace")
	}
}
