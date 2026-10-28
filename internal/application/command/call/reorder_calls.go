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

// ReorderCallsCommand sets call positions from a full ordered ID list (DnD).
type ReorderCallsCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallIDs    []uuid.UUID
}

type ReorderCallsHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewReorderCallsHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *ReorderCallsHandler {
	return &ReorderCallsHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *ReorderCallsHandler) Handle(ctx context.Context, cmd ReorderCallsCommand) ([]domaincall.Call, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	var ordered []domaincall.Call
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		if err := w.ReorderCalls(cmd.CallIDs, globals); err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		ordered = append([]domaincall.Call{}, w.Calls...)
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return ordered, nil
}
