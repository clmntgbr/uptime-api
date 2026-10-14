package di

import (
	"log"
	"log/slog"
	"time"

	authcmd "go-api/internal/application/command/auth"
	callcmd "go-api/internal/application/command/call"
	identitycmd "go-api/internal/application/command/identity"
	schedulecmd "go-api/internal/application/command/schedule"
	usercmd "go-api/internal/application/command/user"
	workflowcmd "go-api/internal/application/command/workflow"
	querycall "go-api/internal/application/query/call"
	queryuser "go-api/internal/application/query/user"
	queryworkflow "go-api/internal/application/query/workflow"
	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/centrifugo"
	infraClerk "go-api/internal/infrastructure/clerk"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/messaging/kafka"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/processed"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/persistence/write"
	httphandler "go-api/internal/interfaces/http/handler"
	"go-api/internal/interfaces/http/middleware"

	"gorm.io/gorm"
)

type Container struct {
	AuthenticateMiddleware *middleware.AuthenticateMiddleware
	UserWebhookMiddleware  *middleware.UserWebhookMiddleware
	UserWebhookHandler     *httphandler.UserWebhookHandler
	UserHandler            *httphandler.UserHandler
	RealtimeHandler        *httphandler.RealtimeHandler
	WorkflowHandler        *httphandler.WorkflowHandler
	CallHandler            *httphandler.CallHandler
	KafkaProducer          *kafka.Producer
}

func NewContainer(db *gorm.DB, env *config.Config) *Container {
	jwksProvider, err := infraClerk.NewJWKSProvider(env)
	if err != nil {
		log.Fatalf("failed to create JWKS provider: %v", err)
	}

	userWriteRepo := write.NewUserWriteRepository(db)
	userReadRepo := read.NewUserReadRepository(db)
	outboxRepo := outbox.NewRepository(db)

	createUserHandler := usercmd.NewCreateUserHandler(userWriteRepo, outboxRepo)
	updateUserHandler := usercmd.NewUpdateUserHandler(userWriteRepo, outboxRepo)
	getUserByExternalIDHandler := usercmd.NewGetUserByExternalIDHandler(userWriteRepo)
	deleteUserByExternalIDHandler := usercmd.NewDeleteUserByExternalIDHandler(userWriteRepo, outboxRepo)
	validateTokenHandler := authcmd.NewValidateTokenHandler(jwksProvider, userWriteRepo)
	fetchUserHandler := identitycmd.NewFetchUserHandler(infraClerk.NewUserGateway(env.ClerkSecretKey))
	getUserByIDHandler := queryuser.NewGetUserByIDHandler(userReadRepo)

	cipher, err := crypto.NewAESCipher(env.SecretsEncryptionKey)
	if err != nil {
		log.Fatalf("invalid SECRETS_ENCRYPTION_KEY: %v", err)
	}
	sealer, err := crypto.NewSealer(env.JobSealPublicKey)
	if err != nil {
		log.Fatalf("invalid JOB_SEAL_PUBLIC_KEY: %v", err)
	}
	producer, err := kafka.NewProducer(env.Kafka)
	if err != nil {
		log.Fatalf("failed to create kafka producer: %v", err)
	}

	workflowWriteRepo := write.NewWorkflowWriteRepository(db)
	workflowReadRepo := read.NewWorkflowQueryRepository(db)
	stateRepo := write.NewStateRepository(db)
	globalVariableRepo := write.NewGlobalVariableRepository(db, cipher)
	runWriteRepo := write.NewRunWriteRepository(db)
	clock := port.SystemClock{}

	runManual := schedulecmd.NewRunManualHandler(
		stateRepo,
		workflowWriteRepo,
		globalVariableRepo,
		runWriteRepo,
		processed.NewTxClaimer(db),
		outboxRepo,
		sealer,
		kafka.NewWorkflowPublisher(producer),
		clock,
		time.Minute,
		slog.Default(),
	)

	return &Container{
		AuthenticateMiddleware: middleware.NewAuthenticateMiddleware(
			validateTokenHandler,
			fetchUserHandler,
			createUserHandler,
		),
		UserWebhookMiddleware: middleware.NewUserWebhookMiddleware(env.ClerkWebhookSecret),
		UserWebhookHandler: httphandler.NewUserWebhookHandler(
			getUserByExternalIDHandler,
			createUserHandler,
			updateUserHandler,
			deleteUserByExternalIDHandler,
		),
		UserHandler:     httphandler.NewUserHandler(getUserByIDHandler),
		RealtimeHandler: httphandler.NewRealtimeHandler(centrifugo.NewConnectionInfoCreator(env)),
		WorkflowHandler: httphandler.NewWorkflowHandler(
			workflowcmd.NewCreateWorkflowHandler(workflowWriteRepo, stateRepo, globalVariableRepo, outboxRepo, clock),
			workflowcmd.NewUpdateWorkflowHandler(workflowWriteRepo, stateRepo, globalVariableRepo, outboxRepo, clock),
			queryworkflow.NewListWorkflowsHandler(workflowReadRepo),
			queryworkflow.NewGetWorkflowHandler(workflowReadRepo),
			workflowcmd.NewDeleteWorkflowHandler(workflowWriteRepo, outboxRepo),
			runManual,
		),
		CallHandler: httphandler.NewCallHandler(
			callcmd.NewCreateCallHandler(workflowWriteRepo, globalVariableRepo, outboxRepo),
			callcmd.NewUpdateCallHandler(workflowWriteRepo, globalVariableRepo, outboxRepo),
			callcmd.NewDeleteCallHandler(workflowWriteRepo, globalVariableRepo, outboxRepo),
			querycall.NewListCallsHandler(workflowReadRepo),
			querycall.NewGetCallHandler(workflowReadRepo),
		),
		KafkaProducer: producer,
	}
}
