package handler

import (
	"context"

	queryuser "go-api/internal/application/query/user"
	domainuser "go-api/internal/domain/user"
)

type userGetByIDHandler interface {
	Handle(ctx context.Context, q queryuser.GetUserByIDQuery) (*domainuser.UserView, error)
}
