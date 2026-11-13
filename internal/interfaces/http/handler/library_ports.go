package handler

import (
	"context"

	cmdlibrary "go-api/internal/application/command/library"
	querylibrary "go-api/internal/application/query/library"
	domainlibrary "go-api/internal/domain/library"
)

type libraryCreateHandler interface {
	Handle(ctx context.Context, cmd cmdlibrary.CreateItemCommand) (*domainlibrary.Item, error)
}

type libraryUpdateHandler interface {
	Handle(ctx context.Context, cmd cmdlibrary.UpdateItemCommand) (*domainlibrary.Item, error)
}

type libraryDeleteHandler interface {
	Handle(ctx context.Context, cmd cmdlibrary.DeleteItemCommand) error
}

type libraryImportHandler interface {
	Handle(ctx context.Context, cmd cmdlibrary.ImportCommand) ([]domainlibrary.Item, error)
}

type libraryListHandler interface {
	Handle(ctx context.Context, q querylibrary.ListItemsQuery) ([]domainlibrary.ListView, int64, error)
}

type libraryGetHandler interface {
	Handle(ctx context.Context, q querylibrary.GetItemQuery) (*domainlibrary.Item, error)
}

type librarySaveFromCallHandler interface {
	Handle(ctx context.Context, cmd cmdlibrary.SaveFromCallCommand) (*domainlibrary.Item, error)
}

type libraryPreviewImportHandler interface {
	Handle(ctx context.Context, q querylibrary.PreviewImportQuery) (*querylibrary.PreviewImportResult, error)
}
