package call

import (
	"context"
	"fmt"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// InsertFromLibraryCommand copies a library item into a workflow as a regular call.
// Position is 1-based; 0 appends at the end.
type InsertFromLibraryCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	ItemID     uuid.UUID
	Position   int
}

type InsertFromLibraryHandler struct {
	repo      domainworkflow.WriteRepository
	library   domainlibrary.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewInsertFromLibraryHandler(
	repo domainworkflow.WriteRepository,
	library domainlibrary.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *InsertFromLibraryHandler {
	return &InsertFromLibraryHandler{repo: repo, library: library, variables: variables, outbox: outbox}
}

func (h *InsertFromLibraryHandler) Handle(ctx context.Context, cmd InsertFromLibraryCommand) (*domaincall.Call, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	var created domaincall.Call
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.library.FindByID(txCtx, cmd.UserID, cmd.ItemID)
		if err != nil {
			return err
		}
		if item == nil {
			return domainlibrary.ErrNotFound
		}
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		position := cmd.Position
		if position == 0 {
			position = len(w.Calls) + 1
		}
		call, err := w.InsertCall(item.Definition, position, globals)
		if err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		created = call
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}
