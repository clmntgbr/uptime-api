package contract

import (
	"fmt"
	"time"
)

const NotificationRequestV1 = 1

// NotificationRequest is published on notifications.requests (key = workflow_id).
type NotificationRequest struct {
	V             int       `json:"v"`
	Kind          string    `json:"kind"`
	Type          string    `json:"type,omitempty"`
	UserID        string    `json:"user_id"`
	WorkflowID    string    `json:"workflow_id"`
	ItemID        string    `json:"item_id,omitempty"`
	Subject       string    `json:"subject,omitempty"`
	DaysLeft      int       `json:"days_left,omitempty"`
	ThresholdDays int       `json:"threshold_days,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	Severity      string    `json:"severity,omitempty"`
	Message       string    `json:"message"`
}

func (n NotificationRequest) Validate() error {
	if n.V != NotificationRequestV1 {
		return fmt.Errorf("%w: unsupported notification version %d", ErrInvalidNotification, n.V)
	}
	if n.Kind == "" || n.UserID == "" || n.WorkflowID == "" || n.Message == "" {
		return fmt.Errorf("%w: kind, user_id, workflow_id and message are required", ErrInvalidNotification)
	}
	return nil
}

var ErrInvalidNotification = fmt.Errorf("invalid notification")

const NotificationResultV1 = 1

// NotificationResult is published on notifications.results after delivery attempt.
type NotificationResult struct {
	V          int       `json:"v"`
	Kind       string    `json:"kind"`
	WorkflowID string    `json:"workflow_id"`
	ItemID     string    `json:"item_id,omitempty"`
	Status     string    `json:"status"` // sent | failed
	Error      string    `json:"error,omitempty"`
	SentAt     time.Time `json:"sent_at"`
}
