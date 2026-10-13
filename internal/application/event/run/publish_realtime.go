package run

import (
	"context"
	"encoding/json"
	"log/slog"

	"go-api/internal/application/messaging"
	"go-api/internal/application/realtime"
	"go-api/internal/domain/port"
	domainrun "go-api/internal/domain/run"
)

type PublishRunRealtimeHandler struct {
	publisher *realtime.Publisher
	logger    *slog.Logger
}

func NewPublishRunRealtimeHandler(transport port.RealtimePublisher, logger *slog.Logger) *PublishRunRealtimeHandler {
	return &PublishRunRealtimeHandler{publisher: realtime.NewPublisher(transport), logger: logger}
}

// OnRecorded pushes run.completed to the owner. A realtime notification that
// cannot be delivered is dropped rather than retried: the run is already
// stored and the front reloads it on reconnection.
func (h *PublishRunRealtimeHandler) OnRecorded(ctx context.Context, payload []byte) error {
	var evt domainrun.RunRecorded
	if err := json.Unmarshal(payload, &evt); err != nil {
		return messaging.NonRetryable(err)
	}
	if err := h.publisher.ToUser(ctx, realtime.EntityRun, realtime.ActionCompleted, evt.UserID, evt); err != nil {
		h.logger.Warn("run.completed not delivered", "run_id", evt.RunID, "error", err)
	}
	return nil
}
