package di

import (
	"context"
	"errors"
	"fmt"

	appvariable "go-api/internal/application/command/variable"
	appworkflow "go-api/internal/application/command/workflow"
	"go-api/internal/domain/port"
	domainuser "go-api/internal/domain/user"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
)

type Container struct {
	users             domainuser.UserWriteRepository
	CreateWorkflow    *appworkflow.CreateWorkflowHandler
	SetGlobalVariable *appvariable.SetGlobalVariableHandler
}

func NewContainer() (*Container, error) {
	env := config.LoadCLI()
	db := config.ConnectDatabaseURL(env.DatabaseURL)

	var cipher port.SecretCipher = missingKeyCipher{}
	if env.SecretsEncryptionKey != "" {
		aes, err := crypto.NewAESCipher(env.SecretsEncryptionKey)
		if err != nil {
			return nil, err
		}
		cipher = aes
	}

	globals := write.NewGlobalVariableRepository(db, cipher)
	workflows := write.NewWorkflowWriteRepository(db, cipher)
	outboxRepo := outbox.NewRepository(db)

	return &Container{
		users:             write.NewUserWriteRepository(db),
		CreateWorkflow:    appworkflow.NewCreateWorkflowHandler(workflows, write.NewStateRepository(db), globals, write.NewExpiryPolicyRepository(db), outboxRepo, port.SystemClock{}),
		SetGlobalVariable: appvariable.NewSetGlobalVariableHandler(globals, workflows),
	}, nil
}

// ResolveUser accepts a user UUID or a Clerk ID.
func (c *Container) ResolveUser(ctx context.Context, ref string) (uuid.UUID, error) {
	var (
		user *domainuser.User
		err  error
	)
	if id, parseErr := uuid.Parse(ref); parseErr == nil {
		user, err = c.users.GetByID(ctx, id)
	} else {
		user, err = c.users.GetByClerkID(ctx, ref)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if user == nil {
		return uuid.Nil, fmt.Errorf("user %q not found", ref)
	}
	return user.ID, nil
}

var errMissingKey = errors.New("SECRETS_ENCRYPTION_KEY is required to read or write secret variables")

type missingKeyCipher struct{}

func (missingKeyCipher) Encrypt(string) (string, error) { return "", errMissingKey }
func (missingKeyCipher) Decrypt(string) (string, error) { return "", errMissingKey }
