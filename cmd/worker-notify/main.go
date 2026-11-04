package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"go-api/internal/contract"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/mail"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/observability"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

	var db *gorm.DB
	if env.DatabaseURL != "" {
		db = config.ConnectDatabaseURL(env.DatabaseURL)
	} else {
		logger.Warn("DATABASE_URL unset: cannot resolve recipient emails")
	}

	sender := mail.NewSender(mail.SMTPConfig{
		Host: env.SMTP.Host, Port: env.SMTP.Port, From: env.SMTP.From,
		User: env.SMTP.User, Password: env.SMTP.Password,
	})
	if !sender.Enabled() {
		logger.Warn("SMTP_HOST unset: notifications will be logged only")
	} else {
		logger.Info("smtp enabled", "host", env.SMTP.Host, "port", env.SMTP.Port, "from", env.SMTP.From)
	}

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

		logger.Info("notification",
			"kind", req.Kind,
			"type", req.Type,
			"workflow_id", req.WorkflowID,
			"user_id", req.UserID,
			"recipients", len(req.Recipients),
			"severity", req.Severity,
			"message", req.Message,
		)

		status := "sent"
		var sendErr string
		if sender.Enabled() {
			recipients, err := resolveRecipients(ctx, db, req)
			if err != nil || len(recipients) == 0 {
				logger.Error("resolve recipients failed", "user_id", req.UserID, "error", err)
				status = "failed"
				sendErr = "no recipients"
			} else {
				subject, body := mail.Render(req)
				for _, to := range recipients {
					if err := sender.Send(to, subject, body); err != nil {
						logger.Error("smtp send failed", "to", to, "kind", req.Kind, "error", err)
						status = "failed"
						sendErr = err.Error()
						break
					}
					logger.Info("smtp sent", "to", to, "kind", req.Kind, "subject", subject)
				}
			}
		}

		return publisher.PublishResult(ctx, contract.NotificationResult{
			V: contract.NotificationResultV1, Kind: req.Kind,
			WorkflowID: req.WorkflowID, ItemID: req.ItemID,
			Status: status, Error: sendErr, SentAt: time.Now().UTC(),
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

func resolveRecipients(ctx context.Context, db *gorm.DB, req contract.NotificationRequest) ([]string, error) {
	if len(req.Recipients) > 0 {
		out := make([]string, 0, len(req.Recipients))
		for _, r := range req.Recipients {
			e := strings.ToLower(strings.TrimSpace(r))
			if e != "" {
				out = append(out, e)
			}
		}
		return out, nil
	}
	if db == nil {
		return nil, nil
	}
	id, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, err
	}
	var email string
	err = db.WithContext(ctx).
		Table("users").
		Select("email").
		Where("id = ?", id).
		Limit(1).
		Scan(&email).Error
	if err != nil || email == "" {
		return nil, err
	}
	return []string{strings.ToLower(strings.TrimSpace(email))}, nil
}
