package incident

import (
	"context"
	"encoding/json"

	"go-api/internal/application/messaging"
	"go-api/internal/application/realtime"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/port"
)

type PublishRealtimeHandler struct {
	publisher *realtime.Publisher
}

func NewPublishRealtimeHandler(realtimePublisher port.RealtimePublisher) *PublishRealtimeHandler {
	return &PublishRealtimeHandler{
		publisher: realtime.NewPublisher(realtimePublisher),
	}
}

type idsOnly struct {
	IncidentID string `json:"incidentId"`
	UserID     string `json:"userId"`
	WorkflowID string `json:"workflowId"`
}

func (h *PublishRealtimeHandler) OnOpened(ctx context.Context, payload []byte) error {
	var evt domainincident.Opened
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityIncident, realtime.ActionOpened, evt.UserID, idsOnly{
		IncidentID: evt.IncidentID, UserID: evt.UserID, WorkflowID: evt.WorkflowID,
	})
}

func (h *PublishRealtimeHandler) OnAcknowledged(ctx context.Context, payload []byte) error {
	var evt domainincident.Acknowledged
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityIncident, realtime.ActionAcknowledged, evt.UserID, idsOnly{
		IncidentID: evt.IncidentID, UserID: evt.UserID, WorkflowID: evt.WorkflowID,
	})
}

func (h *PublishRealtimeHandler) OnResolved(ctx context.Context, payload []byte) error {
	var evt domainincident.Resolved
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityIncident, realtime.ActionResolved, evt.UserID, idsOnly{
		IncidentID: evt.IncidentID, UserID: evt.UserID, WorkflowID: evt.WorkflowID,
	})
}

func (h *PublishRealtimeHandler) OnReopened(ctx context.Context, payload []byte) error {
	var evt domainincident.Reopened
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityIncident, realtime.ActionReopened, evt.UserID, idsOnly{
		IncidentID: evt.IncidentID, UserID: evt.UserID, WorkflowID: evt.WorkflowID,
	})
}
