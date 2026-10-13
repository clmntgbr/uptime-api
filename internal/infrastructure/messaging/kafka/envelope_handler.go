package kafka

import (
	"context"
	"encoding/json"
	"errors"

	"go-api/internal/application/messaging"
	"go-api/internal/application/registry"
	"go-api/internal/domain/port"
)

var errInvalidEnvelope = errors.New("invalid event envelope: eventId and type are required")

// RegistryHandler decodes outbox envelopes from the `events` topic and
// dispatches them to every handler registered for their type.
func RegistryHandler(reg *registry.HandlerRegistry) Handler {
	return func(ctx context.Context, msg Message) error {
		var envelope port.EventEnvelope
		if err := json.Unmarshal(msg.Value, &envelope); err != nil {
			return messaging.NonRetryable(err)
		}
		if envelope.EventID == "" || envelope.Type == "" {
			return messaging.NonRetryable(errInvalidEnvelope)
		}
		return reg.Dispatch(ctx, envelope.Type, envelope.Payload)
	}
}

// PayloadHandler hands the raw value to a handler that decodes its own
// contract (jobs, results).
func PayloadHandler(fn func(ctx context.Context, payload []byte) error) Handler {
	return func(ctx context.Context, msg Message) error {
		return fn(ctx, msg.Value)
	}
}
