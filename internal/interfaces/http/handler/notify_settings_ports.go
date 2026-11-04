package handler

import (
	"context"

	cmdnotify "go-api/internal/application/command/notifysettings"
	querynotify "go-api/internal/application/query/notifysettings"
	domainnotify "go-api/internal/domain/notifysettings"
)

type getNotifySettingsHandler interface {
	Handle(ctx context.Context, q querynotify.GetQuery) (domainnotify.Settings, error)
}

type upsertNotifySettingsHandler interface {
	Handle(ctx context.Context, cmd cmdnotify.UpsertCommand) (domainnotify.Settings, error)
}
