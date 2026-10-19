package contract

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	CancelVersion = 1
	CancelType    = "workflow.cancel"
)

// Cancel asks worker-calls to skip remaining calls of a run after the current one.
type Cancel struct {
	V           int       `json:"v"`
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	UserID      string    `json:"user_id"`
	WorkflowID  string    `json:"workflow_id"`
	RunID       string    `json:"run_id"`
	RequestedAt time.Time `json:"requested_at"`
}

func (c Cancel) Validate() error {
	if c.V != CancelVersion {
		return fmt.Errorf("%w: unsupported cancel version %d", ErrInvalidJob, c.V)
	}
	if c.Type != CancelType {
		return fmt.Errorf("%w: unexpected type %q", ErrInvalidJob, c.Type)
	}
	for field, value := range map[string]string{
		"id": c.ID, "user_id": c.UserID, "workflow_id": c.WorkflowID, "run_id": c.RunID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s is not a uuid", ErrInvalidJob, field)
		}
	}
	return nil
}
