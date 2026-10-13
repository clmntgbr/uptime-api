package di

import (
	"fmt"
	"log/slog"
	"time"

	"go-api/internal/application/command/schedule"
	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/processed"
	"go-api/internal/infrastructure/persistence/write"

	"gorm.io/gorm"
)

type Container struct {
	Dispatch *schedule.DispatchDueRunsHandler
	Maintain *schedule.MaintainResultsHandler
	Producer *kafka.Producer
}

func NewContainer(db *gorm.DB, env *config.SchedulerConfig, logger *slog.Logger) (*Container, error) {
	cipher, err := crypto.NewAESCipher(env.SecretsEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("secrets cipher: %w", err)
	}
	sealer, err := crypto.NewSealer(env.JobSealPublicKey)
	if err != nil {
		return nil, fmt.Errorf("job sealer: %w", err)
	}
	producer, err := kafka.NewProducer(env.Kafka)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	clock := port.SystemClock{}

	dispatch := schedule.NewDispatchDueRunsHandler(
		write.NewStateRepository(db),
		write.NewWorkflowWriteRepository(db),
		write.NewGlobalVariableRepository(db, cipher),
		write.NewRunWriteRepository(db),
		processed.NewTxClaimer(db),
		outbox.NewRepository(db),
		sealer,
		kafka.NewWorkflowPublisher(producer),
		clock,
		schedule.DispatchConfig{BatchSize: env.DispatchBatchSize, LeaseMargin: env.LeaseMargin, MaxJitter: env.MaxJitter},
		logger,
	)
	maintain := schedule.NewMaintainResultsHandler(
		write.NewRunRetention(db),
		clock,
		time.Duration(env.RetentionDays)*24*time.Hour,
		logger,
	)
	return &Container{Dispatch: dispatch, Maintain: maintain, Producer: producer}, nil
}
