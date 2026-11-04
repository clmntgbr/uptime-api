package incident

import (
	"context"

	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/paginate"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

type ListIncidentsQuery struct {
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	States     []domainincident.State
	Severities []domainincident.Severity
	Query      paginate.PaginateQuery
}

type ListIncidentsHandler struct {
	workflows domainworkflow.ReadRepository
	incidents domainincident.IncidentRepository
}

func NewListIncidentsHandler(
	workflows domainworkflow.ReadRepository,
	incidents domainincident.IncidentRepository,
) *ListIncidentsHandler {
	return &ListIncidentsHandler{workflows: workflows, incidents: incidents}
}

func (h *ListIncidentsHandler) Handle(ctx context.Context, q ListIncidentsQuery) ([]domainincident.Incident, int64, error) {
	wf, err := h.workflows.FindByID(ctx, q.UserID, q.WorkflowID)
	if err != nil {
		return nil, 0, err
	}
	if wf == nil {
		return nil, 0, domainworkflow.ErrNotFound
	}
	return h.incidents.ListByWorkflow(ctx, q.UserID, q.WorkflowID, q.States, q.Severities, q.Query)
}
