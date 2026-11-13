package library

import (
	"context"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"

	"github.com/google/uuid"
)

// Clock is injected so timestamps stay deterministic in tests.
type clock func() time.Time

// CreateItemCommand stores a new library item for the user.
type CreateItemCommand struct {
	UserID     uuid.UUID
	Definition domaincall.Definition
}

type CreateItemHandler struct {
	repo  domainlibrary.WriteRepository
	clock clock
}

func NewCreateItemHandler(repo domainlibrary.WriteRepository) *CreateItemHandler {
	return &CreateItemHandler{repo: repo, clock: time.Now}
}

func (h *CreateItemHandler) Handle(ctx context.Context, cmd CreateItemCommand) (*domainlibrary.Item, error) {
	item, err := domainlibrary.New(cmd.UserID, cmd.Definition, h.clock().UTC())
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItemCommand replaces the definition of an existing item. Calls already
// inserted from the item are not affected.
type UpdateItemCommand struct {
	UserID     uuid.UUID
	ItemID     uuid.UUID
	Definition domaincall.Definition
}

type UpdateItemHandler struct {
	repo  domainlibrary.WriteRepository
	clock clock
}

func NewUpdateItemHandler(repo domainlibrary.WriteRepository) *UpdateItemHandler {
	return &UpdateItemHandler{repo: repo, clock: time.Now}
}

func (h *UpdateItemHandler) Handle(ctx context.Context, cmd UpdateItemCommand) (*domainlibrary.Item, error) {
	item, err := h.repo.FindByID(ctx, cmd.UserID, cmd.ItemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, domainlibrary.ErrNotFound
	}
	if err := item.Replace(cmd.Definition, h.clock().UTC()); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItemCommand removes a library item. Inserted calls keep their definition.
type DeleteItemCommand struct {
	UserID uuid.UUID
	ItemID uuid.UUID
}

type DeleteItemHandler struct {
	repo domainlibrary.WriteRepository
}

func NewDeleteItemHandler(repo domainlibrary.WriteRepository) *DeleteItemHandler {
	return &DeleteItemHandler{repo: repo}
}

func (h *DeleteItemHandler) Handle(ctx context.Context, cmd DeleteItemCommand) error {
	return h.repo.Delete(ctx, cmd.UserID, cmd.ItemID)
}
