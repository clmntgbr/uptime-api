package variable

import (
	"context"

	"github.com/google/uuid"
)

type GlobalRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// FindByName returns nil, nil when the variable does not exist.
	FindByName(ctx context.Context, userID uuid.UUID, name string) (*Global, error)
	Save(ctx context.Context, v *Global) error
	Delete(ctx context.Context, userID uuid.UUID, name string) error
	// ListByUser returns every global of the user with secret values decrypted.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Global, error)
	// Names returns the global names of the user, without reading any value.
	Names(ctx context.Context, userID uuid.UUID) ([]string, error)
}
