package realtime

import (
	"context"
	"log"

	"go-api/internal/application/messaging"
	"go-api/internal/domain/port"

	"github.com/google/uuid"
)

// Publisher pushes domain events to the realtime transport. Event handlers own
// the decoding of their own payloads and delegate the delivery here, so the
// retry classification and logging stay identical across every entity.
type Publisher struct {
	transport port.RealtimePublisher
}

func NewPublisher(transport port.RealtimePublisher) *Publisher {
	return &Publisher{transport: transport}
}

// ToMembers delivers the event to an already-resolved set of members.
func (p *Publisher) ToMembers(
	ctx context.Context,
	entity string,
	action string,
	memberIDs []uuid.UUID,
	payload any,
) error {
	eventType := EventType(entity, action)
	for _, memberID := range memberIDs {
		if err := p.transport.PublishToUser(ctx, memberID, eventType, payload); err != nil {
			log.Printf(
				"centrifugo publish failed type=%s userId=%s: %v",
				eventType, memberID, err,
			)
			return messaging.Retryable(err)
		}
		log.Printf("centrifugo published type=%s userId=%s", eventType, memberID)
	}
	return nil
}

// ToUsers delivers the event to each of the given users (raw UUID strings).
func (p *Publisher) ToUsers(
	ctx context.Context,
	entity string,
	action string,
	userIDsRaw []string,
	payload any,
) error {
	for _, userIDRaw := range userIDsRaw {
		if err := p.ToUser(ctx, entity, action, userIDRaw, payload); err != nil {
			return err
		}
	}
	return nil
}

// ToUser delivers the event to a single user.
func (p *Publisher) ToUser(
	ctx context.Context,
	entity string,
	action string,
	userIDRaw string,
	payload any,
) error {
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return messaging.NonRetryable(err)
	}

	eventType := EventType(entity, action)
	if err := p.transport.PublishToUser(ctx, userID, eventType, payload); err != nil {
		log.Printf("centrifugo publish failed type=%s userId=%s: %v", eventType, userIDRaw, err)
		return messaging.Retryable(err)
	}
	log.Printf("centrifugo published type=%s userId=%s", eventType, userIDRaw)
	return nil
}
