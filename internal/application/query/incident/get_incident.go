package incident

import (
	"context"

	domainincident "go-api/internal/domain/incident"

	"github.com/google/uuid"
)

type GetIncidentQuery struct {
	UserID     uuid.UUID
	IncidentID uuid.UUID
}

type IncidentDetail struct {
	Incident domainincident.Incident
	Events   []domainincident.Event
}

type GetIncidentHandler struct {
	incidents domainincident.IncidentRepository
	events    domainincident.EventRepository
}

func NewGetIncidentHandler(
	incidents domainincident.IncidentRepository,
	events domainincident.EventRepository,
) *GetIncidentHandler {
	return &GetIncidentHandler{incidents: incidents, events: events}
}

func (h *GetIncidentHandler) Handle(ctx context.Context, q GetIncidentQuery) (*IncidentDetail, error) {
	inc, err := h.incidents.GetByID(ctx, q.UserID, q.IncidentID)
	if err != nil {
		return nil, err
	}
	if inc == nil {
		return nil, domainincident.ErrNotFound
	}
	events, err := h.events.ListByIncident(ctx, inc.ID)
	if err != nil {
		return nil, err
	}
	return &IncidentDetail{Incident: *inc, Events: events}, nil
}
