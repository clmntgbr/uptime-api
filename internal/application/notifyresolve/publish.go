package notifyresolve

import (
	"context"
	"log/slog"

	"go-api/internal/contract"
	domainnotify "go-api/internal/domain/notifysettings"

	"github.com/google/uuid"
)

type Family string

const (
	FamilyIncident Family = "incident"
	FamilyTLS      Family = "tls"
	FamilyRun      Family = "run"
)

// PrepareRequest loads settings and returns false when the family must not send.
func PrepareRequest(
	ctx context.Context,
	repo domainnotify.Repository,
	userID, workflowID uuid.UUID,
	family Family,
	req contract.NotificationRequest,
	logger *slog.Logger,
) (contract.NotificationRequest, bool) {
	if repo == nil {
		return req, true
	}
	settings, err := Resolve(ctx, repo, userID, workflowID)
	if err != nil {
		if logger != nil {
			logger.Error("resolve notification settings failed", "workflow_id", workflowID, "error", err)
		}
		return req, false
	}
	switch family {
	case FamilyIncident:
		if !settings.AllowsIncident() {
			return req, false
		}
	case FamilyTLS:
		if !settings.AllowsTLS() {
			return req, false
		}
	case FamilyRun:
		// caller already checked AllowsRun
		if len(settings.Emails) == 0 {
			return req, false
		}
	default:
		if len(settings.Emails) == 0 {
			return req, false
		}
	}
	req.Recipients = append([]string(nil), settings.Emails...)
	return req, true
}

// ShouldNotifyRun reports whether a finished run should emit a notification.
func ShouldNotifyRun(
	ctx context.Context,
	repo domainnotify.Repository,
	userID, workflowID uuid.UUID,
	status, previousStatus string,
) (domainnotify.Settings, bool, error) {
	if repo == nil {
		return domainnotify.Settings{}, false, nil
	}
	settings, err := Resolve(ctx, repo, userID, workflowID)
	if err != nil {
		return domainnotify.Settings{}, false, err
	}
	return settings, settings.AllowsRun(status, previousStatus), nil
}
