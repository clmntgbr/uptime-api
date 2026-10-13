package event

import "github.com/google/uuid"

// PerformedBy is an optional attribution payload for domain events.
type PerformedBy struct {
	PerformedByUserID *string `json:"performedByUserId,omitempty"`
}

func OptionalUserIDString(id uuid.UUID) *string {
	if id == uuid.Nil {
		return nil
	}
	s := id.String()
	return &s
}
