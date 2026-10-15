package variable

import (
	"context"

	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

func loadOwnedWorkflow(
	ctx context.Context,
	repo domainworkflow.WriteRepository,
	userID, workflowID uuid.UUID,
) (*domainworkflow.Workflow, error) {
	w, err := repo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, domainworkflow.ErrNotFound
	}
	if w.UserID != userID {
		return nil, domainworkflow.ErrWrongOwner
	}
	return w, nil
}

func globalNames(ctx context.Context, repo variable.GlobalRepository, userID uuid.UUID) ([]string, error) {
	return repo.Names(ctx, userID)
}
