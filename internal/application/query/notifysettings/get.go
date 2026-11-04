package notifysettings

import (
	"context"

	domainnotify "go-api/internal/domain/notifysettings"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type GetQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
}

type GetHandler struct {
	workflows domainworkflow.ReadRepository
	settings  domainnotify.Repository
}

func NewGetHandler(
	workflows domainworkflow.ReadRepository,
	settings domainnotify.Repository,
) *GetHandler {
	return &GetHandler{workflows: workflows, settings: settings}
}

func (h *GetHandler) Handle(ctx context.Context, q GetQuery) (domainnotify.Settings, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return domainnotify.Settings{}, err
	}
	if wf == nil {
		return domainnotify.Settings{}, domainworkflow.ErrNotFound
	}
	stored, err := h.settings.Get(ctx, q.WorkflowID)
	if err != nil {
		return domainnotify.Settings{}, err
	}
	if stored != nil {
		return *stored, nil
	}
	return domainnotify.DefaultSettings(q.UserID, q.WorkflowID, ""), nil
}
