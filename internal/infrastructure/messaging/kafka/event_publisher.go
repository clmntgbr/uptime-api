package kafka

import (
	"context"
	"encoding/json"

	"go-api/internal/domain/port"
)

// EventPublisher publishes outbox envelopes on the `events` topic, keyed by aggregate.
type EventPublisher struct {
	producer *Producer
}

var _ port.EventPublisher = (*EventPublisher)(nil)

func NewEventPublisher(producer *Producer) *EventPublisher {
	return &EventPublisher{producer: producer}
}

func (p *EventPublisher) Publish(ctx context.Context, envelope port.EventEnvelope) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicEvents, []byte(envelope.AggregateID), body, map[string]string{
		"type": envelope.Type,
	})
}
