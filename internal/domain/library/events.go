package library

import (
	"time"

	"go-api/internal/domain/event"

	"github.com/google/uuid"
)

const EventTypeImported = "library.imported.v1"

// Imported is emitted once per successful import confirmation (bulk create).
// AggregateID is the user id so ordering holds per owner.
type Imported struct {
	ID        string    `json:"eventId"`
	UserID    string    `json:"userId"`
	Count     int       `json:"count"`
	ItemIDs   []string  `json:"itemIds"`
	Timestamp time.Time `json:"occurredAt"`
}

func (e Imported) EventID() string       { return e.ID }
func (e Imported) EventType() string     { return EventTypeImported }
func (e Imported) AggregateID() string   { return e.UserID }
func (e Imported) OccurredAt() time.Time { return e.Timestamp }

// NewImportedEvent builds the domain event for a finished import.
func NewImportedEvent(userID uuid.UUID, items []Item, at time.Time) event.DomainEvent {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID.String())
	}
	return Imported{
		ID:        uuid.New().String(),
		UserID:    userID.String(),
		Count:     len(items),
		ItemIDs:   ids,
		Timestamp: at,
	}
}
