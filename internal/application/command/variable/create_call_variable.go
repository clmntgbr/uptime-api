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

type CreateCallVariableCommand struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	CallID     uuid.UUID
	Name       string
	Path       string
	Source     string
}

type CreateCallVariableHandler struct {
	repo      domainworkflow.WriteRepository
	variables variable.GlobalRepository
	outbox    port.OutboxRepository
}

func NewCreateCallVariableHandler(
	repo domainworkflow.WriteRepository,
	variables variable.GlobalRepository,
	outbox port.OutboxRepository,
) *CreateCallVariableHandler {
	return &CreateCallVariableHandler{repo: repo, variables: variables, outbox: outbox}
}

func (h *CreateCallVariableHandler) Handle(ctx context.Context, cmd CreateCallVariableCommand) (*domaincall.Extraction, error) {
	globals, err := globalNames(ctx, h.variables, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	var created domaincall.Extraction
	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		w, err := loadOwnedWorkflow(txCtx, h.repo, cmd.UserID, cmd.WorkflowID)
		if err != nil {
			return err
		}
		ext, err := w.AddCallVariable(cmd.CallID, domaincall.Extraction{
			Name:       cmd.Name,
			Source:     domaincall.ExtractionSource(cmd.Source),
			Expression: cmd.Path,
		}, globals)
		if err != nil {
			return err
		}
		if err := h.repo.Update(txCtx, w); err != nil {
			return err
		}
		created = ext
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}
