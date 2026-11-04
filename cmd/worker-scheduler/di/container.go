package di

import (
	"fmt"
	"log/slog"
	"time"

	cmdexpiry "go-api/internal/application/command/expiry"
	cmdincident "go-api/internal/application/command/incident"
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
	Dispatch          *schedule.DispatchDueRunsHandler
	Maintain          *schedule.MaintainResultsHandler
	ExpiryScan        *cmdexpiry.ScanHandler
	ScanNoRun         *cmdincident.ScanNoRunHandler
	ScanAvailability  *cmdincident.ScanAvailabilityHandler
	ScanReminders     *cmdincident.ScanRemindersHandler
	ScanTLS           *cmdincident.ScanTLSHandler
	Notify            *kafka.NotificationPublisher
	Producer          *kafka.Producer
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
		write.NewWorkflowWriteRepository(db, cipher),
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
	expiryPolicyRepo := write.NewExpiryPolicyRepository(db)
	expiryItemRepo := write.NewExpiryItemRepository(db)
	expiryEvaluate := cmdexpiry.NewEvaluateHandler(
		expiryPolicyRepo,
		write.NewExpiryAlertRepository(db),
	)
	expiryScan := cmdexpiry.NewScanHandler(
		expiryItemRepo,
		expiryEvaluate,
		logger,
	)

	stateRepo := write.NewStateRepository(db)
	outboxRepo := outbox.NewRepository(db)
	incidentApply := cmdincident.NewApplySignalsHandler(
		write.NewIncidentPolicyRepository(db),
		write.NewSignalStateRepository(db),
		write.NewIncidentRepository(db),
		write.NewIncidentEventRepository(db),
		outboxRepo,
	)
	scanRepo := write.NewIncidentScanRepository(db)
	scanNoRun := cmdincident.NewScanNoRunHandler(stateRepo, scanRepo, incidentApply, logger)
	scanAvailability := cmdincident.NewScanAvailabilityHandler(stateRepo, scanRepo, incidentApply, logger)
	scanReminders := cmdincident.NewScanRemindersHandler(
		stateRepo,
		write.NewIncidentRepository(db),
		write.NewIncidentEventRepository(db),
		outboxRepo,
		logger,
	)
	scanTLS := cmdincident.NewScanTLSHandler(stateRepo, expiryItemRepo, expiryPolicyRepo, incidentApply, logger)

	return &Container{
		Dispatch: dispatch, Maintain: maintain,
		ExpiryScan: expiryScan,
		ScanNoRun: scanNoRun, ScanAvailability: scanAvailability,
		ScanReminders: scanReminders, ScanTLS: scanTLS,
		Notify:   kafka.NewNotificationPublisher(producer),
		Producer: producer,
	}, nil
}
