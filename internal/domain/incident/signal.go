package incident

import (
	"time"

	"github.com/google/uuid"
)

// Signal is one failing or healthy observation for a fingerprint.
type Signal struct {
	Kind       Kind
	Failing    bool
	CallID     *uuid.UUID
	Subject    string // call id string or tls host:port; used in fingerprint
	Cause      map[string]any
	RunID      *uuid.UUID
	ObservedAt time.Time // run started_at or observation time
	// Severity overrides the policy severity when non-nil (e.g. tls expired → critical).
	Severity *Severity
}

type SignalState struct {
	WorkflowID            uuid.UUID
	Fingerprint           string
	ConsecutiveFailures   int
	ConsecutiveSuccesses  int
	LastRunID             *uuid.UUID
	LastSignalAt          *time.Time
	UpdatedAt             time.Time
}

// NextCounters updates consecutive fail/success after one signal.
func NextCounters(failing bool, failures, successes int) (int, int) {
	if failing {
		return failures + 1, 0
	}
	return 0, successes + 1
}

// ShouldSkipSignal is true when the signal is a redelivery or older than last applied.
func ShouldSkipSignal(state SignalState, runID *uuid.UUID, observedAt time.Time) bool {
	if runID != nil && state.LastRunID != nil && *runID == *state.LastRunID {
		return true
	}
	if state.LastSignalAt != nil && !observedAt.After(*state.LastSignalAt) {
		return true
	}
	return false
}

func ShouldOpen(failures, openAfter int) bool {
	return failures >= openAfter
}

func ShouldClose(successes, closeAfter int) bool {
	return successes >= closeAfter
}

// ShouldReopen is true when a failure arrives within ReopenWindow of resolved_at.
func ShouldReopen(resolvedAt *time.Time, now time.Time) bool {
	if resolvedAt == nil {
		return false
	}
	return !now.After(resolvedAt.Add(ReopenWindow))
}
