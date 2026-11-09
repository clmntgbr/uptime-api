package call

import (
	"context"

	"github.com/google/uuid"
)

// MapField is a JSONB object column on calls used for suggestions.
type MapField string

const (
	MapHeaders MapField = "headers"
	MapQuery   MapField = "query"

	DefaultSuggestionLimit = 20
	MaxSuggestionLimit     = 50
)

// SuggestionsRepository searches distinct header/query keys and values across
// all of a user's calls (any workflow).
type SuggestionsRepository interface {
	SearchKeys(ctx context.Context, userID uuid.UUID, field MapField, search string, limit int) ([]string, error)
	SearchValues(ctx context.Context, userID uuid.UUID, field MapField, search string, limit int) ([]string, error)
}
