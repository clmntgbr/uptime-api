package di

import (
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
)

type Container struct {
	Consumer *kafka.Consumer
	Producer *kafka.Producer
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
	if env.AllowPrivateIPs {
		logger.Warn("SSRF protection disabled: private destinations are allowed")
	}

	executor := httpclient.NewExecutor(httpclient.Config{UserAgent: env.UserAgent, AllowPrivate: env.AllowPrivateIPs})
	clock := port.SystemClock{}
	runner := observability.InstrumentRunner(engine.NewRunner(executor, clock, logger))
	handler := execution.NewExecuteJobHandler(opener, runner, kafka.NewWorkflowPublisher(producer), clock, logger)

	consumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.Group, kafka.TopicWorkflowJobs, env.Concurrency, false),
		kafka.PayloadHandler(handler.Handle),
		producer,
	)
	return &Container{Consumer: consumer, Producer: producer}, nil
}
