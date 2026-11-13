package library

import (
	"context"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"

	"github.com/google/uuid"
)

// ImportItem is one reviewed candidate confirmed by the user.
type ImportItem struct {
	Key        string
	Definition domaincall.Definition
}

// ImportCommand stores the confirmed candidates. All or nothing: every item is
// validated before the first write, and the writes share one transaction.
type ImportCommand struct {
	UserID uuid.UUID
	Items  []ImportItem
}

type ImportHandler struct {
	repo  domainlibrary.WriteRepository
	clock clock
}

func NewImportHandler(repo domainlibrary.WriteRepository) *ImportHandler {
	return &ImportHandler{repo: repo, clock: time.Now}
}

func (h *ImportHandler) Handle(ctx context.Context, cmd ImportCommand) ([]domainlibrary.Item, error) {
	now := h.clock().UTC()
	items := make([]domainlibrary.Item, 0, len(cmd.Items))
	for _, in := range cmd.Items {
		item, err := domainlibrary.New(cmd.UserID, in.Definition, now)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	err := h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		for i := range items {
			if err := h.repo.Save(txCtx, &items[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
