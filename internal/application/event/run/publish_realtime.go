package run

import (
	"context"
	"encoding/json"

	"go-api/internal/application/messaging"
	"go-api/internal/application/realtime"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
)

type PublishRealtimeHandler struct {
	publisher *realtime.Publisher
}

func NewPublishRealtimeHandler(realtimePublisher port.RealtimePublisher) *PublishRealtimeHandler {
	return &PublishRealtimeHandler{
		publisher: realtime.NewPublisher(realtimePublisher),
	}
}

func (h *PublishRealtimeHandler) OnStarted(ctx context.Context, payload []byte) error {
	var evt domainrun.RunStarted
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityWorkflow, realtime.ActionStarted, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnCallStarted(ctx context.Context, payload []byte) error {
	var evt domainrun.CallStarted
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityCall, realtime.ActionStarted, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnCallFinished(ctx context.Context, payload []byte) error {
	var evt domainrun.CallFinished
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityCall, realtime.ActionFinished, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnRecorded(ctx context.Context, payload []byte) error {
	var evt domainrun.RunRecorded
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityWorkflow, realtime.ActionFinished, evt.UserID, evt)
}
