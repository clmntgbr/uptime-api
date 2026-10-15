package variable

import (
	"context"
	"fmt"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/port"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type UpdateCallVariableCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
	Name       string
	Path       string
	Source     string
	Secret     bool
}

type UpdateCallVariableHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewUpdateCallVariableHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *UpdateCallVariableHandler {
	return &UpdateCallVariableHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *UpdateCallVariableHandler) Handle(ctx context.Context, cmd UpdateCallVariableCommand) (*domaincall.Extraction, error) {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	var updated domaincall.Extraction
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		ext, err := w.UpdateCallVariable(cmd.CallID, cmd.Name, domaincall.Extraction{
			Name:       cmd.Name,
			Source:     domaincall.ExtractionSource(cmd.Source),
			Expression: cmd.Path,
			Secret:     cmd.Secret,
		}, globals)
		if err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		updated = ext
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
