package handler

import (
	"context"

	queryfeed "go-api/internal/application/query/feed"
)

type feedListHandler interface {
	Handle(ctx context.Context, q queryfeed.ListFeedQuery) (queryfeed.ListFeedResult, error)
}
