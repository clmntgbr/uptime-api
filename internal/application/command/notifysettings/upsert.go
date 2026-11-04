package notifysettings

import (
	"context"
	"fmt"
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
	RunsEnabled      bool
	RunStatuses       []string
	RunMode          string
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
	statuses, err := domainnotify.ParseRunStatuses(cmd.RunStatuses)
	if err != nil {
		return domainnotify.Settings{}, err
	}
	mode := domainnotify.RunModeTransition
	if cmd.RunMode != "" {
		mode, err = domainnotify.ParseRunMode(cmd.RunMode)
		if err != nil {
			return domainnotify.Settings{}, err
		}
	}
	if cmd.RunsEnabled && len(statuses) == 0 {
		return domainnotify.Settings{}, fmt.Errorf("%w: select at least one run status when runs are enabled", domainnotify.ErrInvalid)
	}
	s := domainnotify.Settings{
		WorkflowID: cmd.WorkflowID, UserID: cmd.UserID, Emails: emails,
		IncidentsEnabled: cmd.IncidentsEnabled, TLSEnabled: cmd.TLSEnabled,
		RunsEnabled: cmd.RunsEnabled, RunStatuses: statuses, RunMode: mode,
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.settings.Upsert(ctx, s); err != nil {
		return domainnotify.Settings{}, err
	}
	return s, nil
}
