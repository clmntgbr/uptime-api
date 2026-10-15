package variable

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type CreateStaticCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Name       string
	Value      string
	Secret     bool
}

type CreateStaticHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewCreateStaticHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *CreateStaticHandler {
	return &CreateStaticHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *CreateStaticHandler) Handle(ctx context.Context, cmd CreateStaticCommand) (*domainworkflow.StaticVariable, error) {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	var created domainworkflow.StaticVariable
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		s, err := w.AddStatic(cmd.Name, cmd.Value, cmd.Secret, globals)
		if err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		created = s
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}
