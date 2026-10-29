package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"go-api/internal/contract"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/observability"
)

func main() {
	logger := observability.NewLogger("worker-notify")
	env := config.LoadWorkerNotify()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	producer, err := kafka.NewProducer(env.Kafka)
	if err != nil {
		logger.Error("kafka producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	publisher := kafka.NewNotificationPublisher(producer)
	handler := func(ctx context.Context, payload []byte) error {
		var req contract.NotificationRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			logger.Error("decode notification", "error", err)
			return nil
		}
		if err := req.Validate(); err != nil {
			logger.Error("invalid notification", "error", err)
			return nil
		}
		// Phase 1: log only — real channels come later.
		logger.Info("notification",
			"kind", req.Kind,
			"type", req.Type,
			"workflow_id", req.WorkflowID,
			"subject", req.Subject,
			"severity", req.Severity,
			"message", req.Message,
			"days_left", req.DaysLeft,
			"threshold_days", req.ThresholdDays,
		)
		return publisher.PublishResult(ctx, contract.NotificationResult{
			V: contract.NotificationResultV1, Kind: req.Kind,
			WorkflowID: req.WorkflowID, ItemID: req.ItemID,
			Status: "sent", SentAt: time.Now().UTC(),
		})
	}

	consumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.Group, kafka.TopicNotificationsRequests, env.Concurrency, true),
		kafka.PayloadHandler(handler),
		producer,
	)

	logger.Info("worker-notify started", "concurrency", env.Concurrency)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return consumer.Run(gctx) })
	if err := g.Wait(); err != nil && gctx.Err() == nil {
		logger.Error("worker-notify stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("worker-notify stopped")
}
