package library

import (
	"context"

	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
)

type ListItemsQuery struct {
	UserID uuid.UUID
	Query  paginate.PaginateQuery
}

type ListItemsHandler struct {
	repo domainlibrary.ReadRepository
}

func NewListItemsHandler(repo domainlibrary.ReadRepository) *ListItemsHandler {
	return &ListItemsHandler{repo: repo}
}

func (h *ListItemsHandler) Handle(ctx context.Context, q ListItemsQuery) ([]domainlibrary.ListView, int64, error) {
	return h.repo.List(ctx, q.UserID, q.Query)
}

type GetItemQuery struct {
	UserID uuid.UUID
	ItemID uuid.UUID
}

type GetItemHandler struct {
	repo domainlibrary.WriteRepository
}

// GetItemHandler reads through the write repository, like the variable queries do:
// a library item is small and the aggregate is what the caller needs.
func NewGetItemHandler(repo domainlibrary.WriteRepository) *GetItemHandler {
	return &GetItemHandler{repo: repo}
}

func (h *GetItemHandler) Handle(ctx context.Context, q GetItemQuery) (*domainlibrary.Item, error) {
	item, err := h.repo.FindByID(ctx, q.UserID, q.ItemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, domainlibrary.ErrNotFound
	}
	return item, nil
}
