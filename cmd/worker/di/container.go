package di

import (
	"log"
	"log/slog"

	cmdexpiry "go-api/internal/application/command/expiry"
	commandrun "go-api/internal/application/command/run"
	"go-api/internal/application/event/dedup"
	eventrun "go-api/internal/application/event/run"
	eventuser "go-api/internal/application/event/user"
	eventworkflow "go-api/internal/application/event/workflow"
	"go-api/internal/application/registry"
	domainrun "go-api/internal/domain/run"
	domainuser "go-api/internal/domain/user"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/centrifugo"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/notification"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/processed"
	"go-api/internal/infrastructure/persistence/write"

	"gorm.io/gorm"
)

type Container struct {
	Relay           *outbox.Relay
	EventsConsumer  *kafka.Consumer
	ResultsConsumer *kafka.Consumer
	Producer        *kafka.Producer
}

func NewContainer(db *gorm.DB, env *config.Config) *Container {
	producer, err := kafka.NewProducer(env.Kafka)
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}
	cipher, err := crypto.NewAESCipher(env.SecretsEncryptionKey)
	if err != nil {
		log.Fatalf("invalid SECRETS_ENCRYPTION_KEY: %v", err)
	}

	outboxRepo := outbox.NewRepository(db)
	relay := outbox.NewRelay(outboxRepo, kafka.NewEventPublisher(producer), env.OutboxPollInterval, 50)

	dedupRepo := processed.NewRepository(db)
	notifier := notification.NewLogNotifier()
	realtimePublisher := centrifugo.NewPublisher(env)
	publishUserRealtime := eventuser.NewPublishRealtimeHandler(realtimePublisher)
	publishWorkflowRealtime := eventworkflow.NewPublishRealtimeHandler(realtimePublisher)
	publishRunRealtime := eventrun.NewPublishRealtimeHandler(realtimePublisher)
	reg := registry.NewHandlerRegistry()

	reg.Register(domainuser.EventTypeUserCreated, dedup.With(
		dedupRepo,
		"user_created",
		eventuser.NewUserCreatedHandler().Handle,
	))
	reg.Register(domainuser.EventTypeUserCreated, dedup.With(
		dedupRepo,
		"notify_user_on_created",
		eventuser.NewNotifyUserOnCreatedHandler(notifier).Handle,
	))
	reg.Register(domainuser.EventTypeUserCreated, dedup.With(
		dedupRepo,
		"publish_user_created_realtime",
		publishUserRealtime.OnCreated,
	))
	reg.Register(domainuser.EventTypeUserUpdated, dedup.With(
		dedupRepo,
		"user_updated",
		eventuser.NewUserUpdatedHandler().Handle,
	))
	reg.Register(domainuser.EventTypeUserUpdated, dedup.With(
		dedupRepo,
		"publish_user_updated_realtime",
		publishUserRealtime.OnUpdated,
	))
	reg.Register(domainuser.EventTypeUserDeleted, dedup.With(
		dedupRepo,
		"user_deleted",
		eventuser.NewUserDeletedHandler().Handle,
	))
	reg.Register(domainuser.EventTypeUserDeleted, dedup.With(
		dedupRepo,
		"publish_user_deleted_realtime",
		publishUserRealtime.OnDeleted,
	))

	reg.Register(domainworkflow.EventTypeWorkflowCreated, dedup.With(
		dedupRepo,
		"publish_workflow_created_realtime",
		publishWorkflowRealtime.OnCreated,
	))
	reg.Register(domainworkflow.EventTypeWorkflowUpdated, dedup.With(
		dedupRepo,
		"publish_workflow_updated_realtime",
		publishWorkflowRealtime.OnUpdated,
	))
	reg.Register(domainworkflow.EventTypeWorkflowDeleted, dedup.With(
		dedupRepo,
		"publish_workflow_deleted_realtime",
		publishWorkflowRealtime.OnDeleted,
	))

	reg.Register(domainrun.EventTypeRunStarted, dedup.With(
		dedupRepo,
		"publish_run_started_realtime",
		publishRunRealtime.OnStarted,
	))
	reg.Register(domainrun.EventTypeCallStarted, dedup.With(
		dedupRepo,
		"publish_call_started_realtime",
		publishRunRealtime.OnCallStarted,
	))
	reg.Register(domainrun.EventTypeCallFinished, dedup.With(
		dedupRepo,
		"publish_call_finished_realtime",
		publishRunRealtime.OnCallFinished,
	))
	reg.Register(domainrun.EventTypeRunProgressed, dedup.With(
		dedupRepo,
		"publish_run_progressed_realtime",
		publishRunRealtime.OnCallFinished,
	))
	reg.Register(domainrun.EventTypeRunRecorded, dedup.With(
		dedupRepo,
		"publish_run_recorded_realtime",
		publishRunRealtime.OnRecorded,
	))

	eventsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.WorkerEventsGroup, kafka.TopicEvents, env.WorkerConcurrency, true),
		kafka.RegistryHandler(reg),
		producer,
	)

	stateRepo := write.NewStateRepository(db)
	runWriteRepo := write.NewRunWriteRepository(db)
	claimer := processed.NewTxClaimer(db)
	expiryEvaluate := cmdexpiry.NewEvaluateHandler(
		write.NewExpiryPolicyRepository(db),
		write.NewExpiryAlertRepository(db),
	)
	expiryRecord := cmdexpiry.NewRecordObservationsHandler(
		write.NewExpiryItemRepository(db),
		expiryEvaluate,
	)
	recordResult := commandrun.NewRecordResultHandler(
		stateRepo,
		write.NewWorkflowWriteRepository(db, cipher),
		runWriteRepo,
		write.NewCertificateRepository(db),
		expiryRecord,
		kafka.NewNotificationPublisher(producer),
		claimer,
		outboxRepo,
		slog.Default(),
	)
	recordProgress := commandrun.NewRecordCallProgressHandler(stateRepo, runWriteRepo, claimer, outboxRepo)
	resultsRouter := commandrun.NewResultsRouter(recordResult, recordProgress)
	resultsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.WorkerResultsGroup, kafka.TopicWorkflowResults, env.WorkerConcurrency, true),
		kafka.PayloadHandler(resultsRouter.Handle),
		producer,
	)

	return &Container{
		Relay:           relay,
		EventsConsumer:  eventsConsumer,
		ResultsConsumer: resultsConsumer,
		Producer:        producer,
	}
}
