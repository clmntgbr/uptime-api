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

type CreateCallCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Definition domaincall.Definition
}

type CreateCallHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewCreateCallHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *CreateCallHandler {
	return &CreateCallHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *CreateCallHandler) Handle(ctx context.Context, cmd CreateCallCommand) (*domaincall.Call, error) {
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
		call, err := w.AddCall(cmd.Definition, globals)
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
