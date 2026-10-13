package port

import (
	"context"
	"time"

	"go-api/internal/contract"
)

// JobPublisher publishes self-contained run jobs on `workflow.jobs`.
type JobPublisher interface {
	PublishJob(ctx context.Context, job contract.Job) error
}

// ResultPublisher publishes run results on `workflow.results`.
type ResultPublisher interface {
	PublishResult(ctx context.Context, result contract.Result) error
}

// WorkflowRunner executes a job end to end and returns its result. It never
// fails: execution problems are part of the result (call status `error`).
type WorkflowRunner interface {
	Run(ctx context.Context, job contract.Job, secrets map[string]string) contract.Result
}

// BodyStore keeps response bodies too large for the database or a Kafka message.
type BodyStore interface {
	// Put stores data under key and returns a reference such as s3://bucket/key.
	Put(ctx context.Context, key string, data []byte, contentType string) (string, error)
}

// SecretCipher encrypts secret values at rest in the database.
type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// JobSealer seals secret values for the worker-calls key: only worker-calls can open them.
type JobSealer interface {
	Seal(secrets map[string]string) (*contract.SealedSecrets, error)
}

// JobOpener opens sealed job secrets, in memory, for the time of an execution.
type JobOpener interface {
	Open(sealed *contract.SealedSecrets) (map[string]string, error)
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
