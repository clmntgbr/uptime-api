package handler

import (
	"context"

	cmdincident "go-api/internal/application/command/incident"
	queryincident "go-api/internal/application/query/incident"
	domainincident "go-api/internal/domain/incident"
)

type incidentListPoliciesHandler interface {
	Handle(ctx context.Context, q queryincident.ListPoliciesQuery) ([]domainincident.Policy, error)
}

type incidentUpsertPolicyHandler interface {
	Handle(ctx context.Context, cmd cmdincident.UpsertPolicyCommand) (domainincident.Policy, error)
}

type incidentListHandler interface {
	Handle(ctx context.Context, q queryincident.ListIncidentsQuery) ([]domainincident.Incident, int64, error)
}

type incidentGetHandler interface {
	Handle(ctx context.Context, q queryincident.GetIncidentQuery) (*queryincident.IncidentDetail, error)
}

type incidentAcknowledgeHandler interface {
	Handle(ctx context.Context, cmd cmdincident.AcknowledgeCommand) (domainincident.Incident, error)
}
