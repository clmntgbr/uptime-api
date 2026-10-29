package handler

import (
	"context"

	cmdexpiry "go-api/internal/application/command/expiry"
	queryexpiry "go-api/internal/application/query/expiry"
	domainexpiry "go-api/internal/domain/expiry"
)

type expiryListPoliciesHandler interface {
	Handle(ctx context.Context, q queryexpiry.ListPoliciesQuery) ([]domainexpiry.Policy, error)
}

type expiryUpsertPolicyHandler interface {
	Handle(ctx context.Context, cmd cmdexpiry.UpsertPolicyCommand) (domainexpiry.Policy, error)
}

type expiryListItemsHandler interface {
	Handle(ctx context.Context, q queryexpiry.ListItemsQuery) ([]domainexpiry.ItemView, error)
}
