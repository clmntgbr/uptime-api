package user

import (
	"context"
	"fmt"

	"go-api/internal/domain/port"
	domainuser "go-api/internal/domain/user"
)

type CreateUserCommand struct {
	ClerkID   string
	FirstName string
	LastName  string
	Email     string
	Banned    bool
}

type CreateUserHandler struct {
	repo   domainuser.UserWriteRepository
	outbox port.OutboxRepository
}

func NewCreateUserHandler(repo domainuser.UserWriteRepository, outbox port.OutboxRepository) *CreateUserHandler {
	return &CreateUserHandler{repo: repo, outbox: outbox}
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (*domainuser.User, error) {
	u := domainuser.NewUser(cmd.ClerkID, cmd.FirstName, cmd.LastName, cmd.Email, cmd.Banned)

	err := h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, u); err != nil {
			return err
		}
		return h.outbox.StoreEvents(txCtx, u.PullEvents())
	})
	if err != nil {
		// Two concurrent first logins (or a webhook racing the middleware) both try
		// to insert the same clerk_id. The loser gets a unique violation: return the
		// row the winner created instead of failing the request.
		if existing, lookupErr := h.repo.GetByClerkID(ctx, cmd.ClerkID); lookupErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}
