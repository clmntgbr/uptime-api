package kafka

import (
	"context"
	"encoding/json"

	"go-api/internal/contract"
)

type NotificationPublisher struct {
	producer *Producer
}

func NewNotificationPublisher(producer *Producer) *NotificationPublisher {
	return &NotificationPublisher{producer: producer}
}

func (p *NotificationPublisher) PublishRequest(ctx context.Context, req contract.NotificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicNotificationsRequests, []byte(req.WorkflowID), body, map[string]string{
		"type": "notification.request.v1",
	})
}

func (p *NotificationPublisher) PublishResult(ctx context.Context, res contract.NotificationResult) error {
	body, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicNotificationsResults, []byte(res.WorkflowID), body, map[string]string{
		"type": "notification.result.v1",
	})
}
