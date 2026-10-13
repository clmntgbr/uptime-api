// Package contract holds the versioned JSON messages exchanged over Kafka.
// It is a leaf package (no internal imports) shared by every binary; a breaking
// change requires a new `v`.
package contract

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	JobVersion = 1
	JobType    = "workflow.job"

	TriggerSchedule = "schedule"
	TriggerManual   = "manual"
	TriggerTest     = "test"
)

// Job is a self-contained run request published on `workflow.jobs`: worker-calls
// executes it without any database access. Secret values only travel sealed.
type Job struct {
	V            int            `json:"v"`
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	UserID       string         `json:"user_id"`
	WorkflowID   string         `json:"workflow_id"`
	WorkflowName string         `json:"workflow_name"`
	RunID        string         `json:"run_id"`
	Trigger      string         `json:"trigger"`
	ScheduledAt  time.Time      `json:"scheduled_at"`
	Deadline     time.Time      `json:"deadline"`
	TimeoutMS    int            `json:"timeout_ms"`
	Variables    []JobVariable  `json:"variables"`
	Secrets      *SealedSecrets `json:"secrets,omitempty"`
	Calls        []JobCall      `json:"calls"`
	EmittedAt    time.Time      `json:"emitted_at"`
}

// JobVariable is a non-secret `global` or `static` variable, in clear.
type JobVariable struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SealedSecrets uses envelope encryption: a random data key encrypts each value
// (secretbox), and the data key is sealed for the worker-calls public key.
type SealedSecrets struct {
	SealedKey string            `json:"sealed_key"`
	Values    map[string]string `json:"values"`
}

type JobCall struct {
	ID          string            `json:"id"`
	Order       int               `json:"order"`
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Body        string            `json:"body,omitempty"`
	TimeoutMS   int               `json:"timeout_ms"`
	VerifyTLS   bool              `json:"verify_tls"`
	IPVersion   string            `json:"ip_version"`
	ProxyURL    string            `json:"proxy_url,omitempty"`
	Extractions []JobExtraction   `json:"extractions,omitempty"`
	Rules       []JobRule         `json:"rules,omitempty"`
}

type JobExtraction struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Expression string `json:"expression,omitempty"`
}

type JobRule struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Target   string `json:"target,omitempty"`
	Value    string `json:"value,omitempty"`
}

var ErrInvalidJob = errors.New("invalid job")

// Validate rejects malformed or unsupported jobs before any execution.
func (j Job) Validate() error {
	if j.V != JobVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidJob, j.V)
	}
	if j.Type != JobType {
		return fmt.Errorf("%w: unexpected type %q", ErrInvalidJob, j.Type)
	}
	for field, value := range map[string]string{
		"id": j.ID, "user_id": j.UserID, "workflow_id": j.WorkflowID, "run_id": j.RunID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s is not a uuid", ErrInvalidJob, field)
		}
	}
	switch j.Trigger {
	case TriggerSchedule, TriggerManual, TriggerTest:
	default:
		return fmt.Errorf("%w: unknown trigger %q", ErrInvalidJob, j.Trigger)
	}
	if len(j.Calls) == 0 {
		return fmt.Errorf("%w: no calls", ErrInvalidJob)
	}
	if j.TimeoutMS <= 0 {
		return fmt.Errorf("%w: timeout_ms must be positive", ErrInvalidJob)
	}
	if j.Deadline.IsZero() {
		return fmt.Errorf("%w: deadline is required", ErrInvalidJob)
	}
	for i, call := range j.Calls {
		if call.Name == "" || call.Method == "" || call.URL == "" {
			return fmt.Errorf("%w: call %d is missing name, method or url", ErrInvalidJob, i)
		}
		if call.TimeoutMS <= 0 {
			return fmt.Errorf("%w: call %q timeout_ms must be positive", ErrInvalidJob, call.Name)
		}
	}
	return nil
}
