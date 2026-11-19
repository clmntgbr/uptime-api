package outbox

import (
	"context"
	"log/slog"
	"time"

	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

type Relay struct {
	outbox    port.OutboxRepository
	publisher port.EventPublisher
	interval  time.Duration
	batchSize int
}

func NewRelay(outbox port.OutboxRepository, publisher port.EventPublisher, interval time.Duration, batchSize int) *Relay {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &Relay{
		outbox:    outbox,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (r *Relay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	slog.Info("outbox relay started", "interval", r.interval.String())

	for {
		select {
		case <-ctx.Done():
			slog.Info("outbox relay stopped")
			return
		case <-ticker.C:
			if err := r.publishBatch(ctx); err != nil {
				slog.Error("outbox relay failed", "error", err)
			}
		}
	}
}

func (r *Relay) publishBatch(ctx context.Context) error {
	messages, err := r.outbox.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	publishedIDs := make([]uuid.UUID, 0, len(messages))
	for _, msg := range messages {
		envelope := port.EventEnvelope{
			EventID:     msg.ID.String(),
			Type:        msg.EventType,
			AggregateID: msg.AggregateID,
			OccurredAt:  msg.CreatedAt.UTC().Format(time.RFC3339Nano),
			Payload:     msg.Payload,
		}
		if err := r.publisher.Publish(ctx, envelope); err != nil {
			slog.Error("publish outbox event failed", "event_id", msg.ID, "error", err)
			break
		}
		publishedIDs = append(publishedIDs, msg.ID)
	}

	if len(publishedIDs) == 0 {
		return nil
	}

	return r.outbox.MarkPublished(ctx, publishedIDs)
}
