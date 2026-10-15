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

func (h *PublishRealtimeHandler) OnProgressed(ctx context.Context, payload []byte) error {
	var evt domainrun.RunProgressed
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityRun, realtime.ActionUpdated, evt.UserID, evt)
}

func (h *PublishRealtimeHandler) OnRecorded(ctx context.Context, payload []byte) error {
	var evt domainrun.RunRecorded
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityRun, realtime.ActionUpdated, evt.UserID, evt)
}
