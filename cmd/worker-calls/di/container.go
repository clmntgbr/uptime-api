package di

import (
	"context"
	"fmt"
	"log/slog"

	"go-api/internal/application/command/execution"
	"go-api/internal/domain/port"
	"go-api/internal/engine"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/httpclient"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/observability"
	"go-api/internal/infrastructure/storage"
)

type Container struct {
	JobsConsumer    *kafka.Consumer
	CancelsConsumer *kafka.Consumer
	Producer        *kafka.Producer
}

func NewContainer(env *config.WorkerCallsConfig, logger *slog.Logger) (*Container, error) {
	opener, err := crypto.NewOpener(env.JobSealPublicKey, env.JobSealPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("job opener: %w", err)
	}
	producer, err := kafka.NewProducer(env.Kafka)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	bodies, err := storage.NewS3Store(context.Background(), env.Storage)
	if err != nil {
		return nil, fmt.Errorf("body store: %w", err)
	}
	if env.AllowPrivateIPs {
		if config.IsProduction(env.Environment) {
			return nil, fmt.Errorf("SSRF_ALLOW_PRIVATE=true is forbidden when GO_ENV=%s", env.Environment)
		}
		logger.Warn("SSRF protection disabled: private destinations are allowed")
	}

	minTLS, err := httpclient.ParseMinTLSVersion(env.MinTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("HTTP_MIN_TLS_VERSION: %w", err)
	}

	cancels := engine.NewMemoryCancellationRegistry()
	executor := httpclient.NewExecutor(httpclient.Config{
		UserAgent:     env.UserAgent,
		AllowPrivate:  env.AllowPrivateIPs,
		MinTLSVersion: minTLS,
	})
	clock := port.SystemClock{}
	publisher := kafka.NewWorkflowPublisher(producer)
	runner := observability.InstrumentRunner(
		engine.NewRunner(executor, bodies, clock, logger).
			WithProgressPublisher(publisher).
			WithCancellationChecker(cancels),
	)
	handler := execution.NewExecuteJobHandler(opener, runner, publisher, clock, logger)
	cancelHandler := execution.NewRecordCancelHandler(cancels)

	jobsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.Group, kafka.TopicWorkflowJobs, env.Concurrency, false),
		kafka.PayloadHandler(handler.Handle),
		producer,
	)
	// Each replica needs every cancel: the registry is in-memory per process.
	// A shared group would deliver the signal to only one instance.
	cancelsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.Group+"-cancels-"+env.Kafka.InstanceID, kafka.TopicWorkflowCancels, 1, false),
		kafka.PayloadHandler(cancelHandler.Handle),
		producer,
	)
	return &Container{
		JobsConsumer:    jobsConsumer,
		CancelsConsumer: cancelsConsumer,
		Producer:        producer,
	}, nil
}
