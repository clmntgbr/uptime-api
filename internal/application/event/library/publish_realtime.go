package library

import (
	"context"
	"encoding/json"

	"go-api/internal/application/messaging"
	"go-api/internal/application/realtime"
	domainlibrary "go-api/internal/domain/library"
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

type importedPayload struct {
	UserID  string   `json:"userId"`
	Count   int      `json:"count"`
	ItemIDs []string `json:"itemIds"`
}

func (h *PublishRealtimeHandler) OnImported(ctx context.Context, payload []byte) error {
	var evt domainlibrary.Imported
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	return h.publisher.ToUser(ctx, realtime.EntityLibrary, realtime.ActionImported, evt.UserID, importedPayload{
		UserID:  evt.UserID,
		Count:   evt.Count,
		ItemIDs: evt.ItemIDs,
	})
}
