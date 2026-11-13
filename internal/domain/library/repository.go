package library

import (
	"context"

	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type WriteRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// Save inserts or replaces the item (keyed by ID).
	Save(ctx context.Context, item *Item) error
	// Delete returns ErrNotFound when the item does not exist for that user.
	Delete(ctx context.Context, userID, id uuid.UUID) error
	// FindByID returns nil, nil when the item does not exist for that user.
	FindByID(ctx context.Context, userID, id uuid.UUID) (*Item, error)
}

type ReadRepository interface {
	// List returns one page of the user's items and the total before pagination.
	List(ctx context.Context, userID uuid.UUID, q paginate.PaginateQuery) ([]ListView, int64, error)
}
