package di

import (
	"log"

	commandrun "go-api/internal/application/command/run"
	"go-api/internal/application/event/dedup"
	eventuser "go-api/internal/application/event/user"
	eventworkflow "go-api/internal/application/event/workflow"
	"go-api/internal/application/registry"
	domainuser "go-api/internal/domain/user"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/centrifugo"
	"go-api/internal/infrastructure/config"
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

	outboxRepo := outbox.NewRepository(db)
	relay := outbox.NewRelay(outboxRepo, kafka.NewEventPublisher(producer), env.OutboxPollInterval, 50)

	dedupRepo := processed.NewRepository(db)
	notifier := notification.NewLogNotifier()
	realtimePublisher := centrifugo.NewPublisher(env)
	publishUserRealtime := eventuser.NewPublishRealtimeHandler(realtimePublisher)
	publishWorkflowRealtime := eventworkflow.NewPublishRealtimeHandler(realtimePublisher)
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

	eventsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.WorkerEventsGroup, kafka.TopicEvents, env.WorkerConcurrency, true),
		kafka.RegistryHandler(reg),
		producer,
	)

	recordResult := commandrun.NewRecordResultHandler(
		write.NewStateRepository(db),
		write.NewRunWriteRepository(db),
		write.NewCertificateRepository(db),
		processed.NewTxClaimer(db),
		outboxRepo,
	)
	resultsConsumer := kafka.NewConsumer(
		env.Kafka,
		kafka.DefaultConsumerOptions(env.WorkerResultsGroup, kafka.TopicWorkflowResults, env.WorkerConcurrency, true),
		kafka.PayloadHandler(recordResult.Handle),
		producer,
	)

	return &Container{
		Relay:           relay,
		EventsConsumer:  eventsConsumer,
		ResultsConsumer: resultsConsumer,
		Producer:        producer,
	}
}
