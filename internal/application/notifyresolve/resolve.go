package notifyresolve

import (
	"context"
	"time"

	domainnotify "go-api/internal/domain/notifysettings"

	"github.com/google/uuid"
)

// Resolve loads stored settings or defaults (empty emails when no row).
func Resolve(ctx context.Context, repo domainnotify.Repository, userID, workflowID uuid.UUID) (domainnotify.Settings, error) {
	stored, err := repo.Get(ctx, workflowID)
	if err != nil {
		return domainnotify.Settings{}, err
	}
	if stored != nil {
		return *stored, nil
	}
	return domainnotify.DefaultSettings(userID, workflowID, "", time.Time{}), nil
}
