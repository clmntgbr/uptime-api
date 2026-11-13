package library

import (
	"context"
	"time"

	domainlibrary "go-api/internal/domain/library"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// SaveFromCallCommand stores the definition of any call the user owns as a
// library item. The call may belong to any of the user's workflows.
type SaveFromCallCommand struct {
	UserID uuid.UUID
	CallID uuid.UUID
}

type SaveFromCallHandler struct {
	workflows domainworkflow.WriteRepository
	library   domainlibrary.WriteRepository
	clock     clock
}

func NewSaveFromCallHandler(
	workflows domainworkflow.WriteRepository,
	library domainlibrary.WriteRepository,
) *SaveFromCallHandler {
	return &SaveFromCallHandler{workflows: workflows, library: library, clock: time.Now}
}

func (h *SaveFromCallHandler) Handle(ctx context.Context, cmd SaveFromCallCommand) (*domainlibrary.Item, error) {
	workflowID, err := h.workflows.FindWorkflowIDByCall(ctx, cmd.UserID, cmd.CallID)
	if err != nil {
		return nil, err
	}
	w, err := h.workflows.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if w == nil || w.UserID != cmd.UserID {
		return nil, domainworkflow.ErrCallNotFound
	}
	def, err := w.CallDefinition(cmd.CallID)
	if err != nil {
		return nil, err
	}
	item, err := domainlibrary.New(cmd.UserID, def, h.clock().UTC())
	if err != nil {
		return nil, err
	}
	if err := h.library.Save(ctx, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
