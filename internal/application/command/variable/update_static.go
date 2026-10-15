package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpdateStaticCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Name       string
	Value      string
}

type UpdateStaticHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewUpdateStaticHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *UpdateStaticHandler {
	return &UpdateStaticHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *UpdateStaticHandler) Handle(ctx context.Context, cmd UpdateStaticCommand) (*domainworkflow.StaticVariable, error) {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	var updated domainworkflow.StaticVariable
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		s, err := w.UpdateStatic(cmd.Name, cmd.Value, globals)
		if err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		updated = s
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
