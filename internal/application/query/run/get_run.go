package run

import (
	"context"
	"errors"

	domainrun "go-api/internal/domain/run"

	"github.com/google/uuid"
)

var ErrRunNotFound = errors.New("run not found")

type GetRunQuery struct {
	UserID uuid.UUID
	RunID  uuid.UUID
}

type GetRunHandler struct {
	repo domainrun.ReadRepository
}

func NewGetRunHandler(repo domainrun.ReadRepository) *GetRunHandler {
	return &GetRunHandler{repo: repo}
}

func (h *GetRunHandler) Handle(ctx context.Context, q GetRunQuery) (*domainrun.DetailView, error) {
	view, err := h.repo.FindByID(ctx, q.UserID, q.RunID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, ErrRunNotFound
	}
	return view, nil
}
