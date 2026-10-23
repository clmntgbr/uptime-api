package call

import (
	"context"
	"fmt"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// CopyCallCommand duplicates an existing call (request fields and rules).
// Extractions are not copied; {{var}} templates keep resolving from earlier calls.
type CopyCallCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
}

type CopyCallHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewCopyCallHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *CopyCallHandler {
	return &CopyCallHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *CopyCallHandler) Handle(ctx context.Context, cmd CopyCallCommand) (*domaincall.Call, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	var created domaincall.Call
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		call, err := w.DuplicateCall(cmd.CallID, globals)
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
