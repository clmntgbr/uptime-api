package workflow

import (
	"context"
	"encoding/json"

	"go-api/internal/application/messaging"
	"go-api/internal/application/realtime"
	"go-api/internal/domain/port"
	domainworkflow "go-api/internal/domain/workflow"
)

type PublishRealtimeHandler struct {
	publisher *realtime.Publisher
}

func NewPublishRealtimeHandler(realtimePublisher port.RealtimePublisher) *PublishRealtimeHandler {
	return &PublishRealtimeHandler{
		publisher: realtime.NewPublisher(realtimePublisher),
	}
}

func (h *PublishRealtimeHandler) OnCreated(ctx context.Context, payload []byte) error {
	var evt domainworkflow.WorkflowCreated
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityWorkflow, realtime.ActionCreated, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnUpdated(ctx context.Context, payload []byte) error {
	var evt domainworkflow.WorkflowUpdated
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityWorkflow, realtime.ActionUpdated, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnDeleted(ctx context.Context, payload []byte) error {
	var evt domainworkflow.WorkflowDeleted
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityWorkflow, realtime.ActionDeleted, evt.UserID, evt)
}
