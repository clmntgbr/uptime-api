package notifysettings

import (
	"context"
	"time"

	domainnotify "go-api/internal/domain/notifysettings"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpsertCommand struct {
	UserID           uuid.UUID
	WorkflowID       uuid.UUID
	Emails           []string
	IncidentsEnabled bool
	TLSEnabled       bool
}

type UpsertHandler struct {
	workflows domainworkflow.ReadRepository
	settings  domainnotify.Repository
}

func NewUpsertHandler(
	workflows domainworkflow.ReadRepository,
	settings domainnotify.Repository,
) *UpsertHandler {
	return &UpsertHandler{workflows: workflows, settings: settings}
}

func (h *UpsertHandler) Handle(ctx context.Context, cmd UpsertCommand) (domainnotify.Settings, error) {
	wf, err := h.workflows.FindByID(ctx, cmd.UserID, cmd.WorkflowID)
	if err != nil {
		return domainnotify.Settings{}, err
	}
	if wf == nil {
		return domainnotify.Settings{}, domainworkflow.ErrNotFound
	}
	emails := domainnotify.NormalizeEmails(cmd.Emails)
	if err := domainnotify.ValidateEmails(emails); err != nil {
		return domainnotify.Settings{}, err
	}
	s := domainnotify.Settings{
		WorkflowID: cmd.WorkflowID, UserID: cmd.UserID, Emails: emails,
		IncidentsEnabled: cmd.IncidentsEnabled, TLSEnabled: cmd.TLSEnabled,
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.settings.Upsert(ctx, s); err != nil {
		return domainnotify.Settings{}, err
	}
	return s, nil
}
