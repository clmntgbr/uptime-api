package notifysettings

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Get(ctx context.Context, workflowID uuid.UUID) (*Settings, error)
	Upsert(ctx context.Context, s Settings) error
}
