package library

import (
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
)

func TestNewImportedEvent(t *testing.T) {
	userID := uuid.New()
	items := []Item{
		{ID: uuid.New(), UserID: userID, Definition: domaincall.Definition{Name: "a"}},
		{ID: uuid.New(), UserID: userID, Definition: domaincall.Definition{Name: "b"}},
	}
	at := time.Now().UTC()
	evt, ok := NewImportedEvent(userID, items, at).(Imported)
	if !ok {
		t.Fatal("expected Imported event")
	}
	if evt.EventType() != EventTypeImported {
		t.Fatalf("type = %q", evt.EventType())
	}
	if evt.AggregateID() != userID.String() || evt.Count != 2 || len(evt.ItemIDs) != 2 {
		t.Fatalf("unexpected event: %+v", evt)
	}
	if evt.ItemIDs[0] != items[0].ID.String() || evt.ItemIDs[1] != items[1].ID.String() {
		t.Fatalf("item ids = %v", evt.ItemIDs)
	}
}
