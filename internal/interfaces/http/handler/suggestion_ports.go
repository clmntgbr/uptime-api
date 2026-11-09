package handler

import (
	"context"

	querycall "go-api/internal/application/query/call"
)

type suggestCallFieldsHandler interface {
	Handle(ctx context.Context, q querycall.SuggestFieldsQuery) ([]string, error)
}
