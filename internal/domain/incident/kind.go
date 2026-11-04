package incident

import (
	"fmt"
	"time"
)

type Kind string

const (
	KindCallFailed       Kind = "call_failed"
	KindWorkflowFailed   Kind = "workflow_failed"
	KindCallSlow         Kind = "call_slow"
	KindChainBroken      Kind = "chain_broken"
	KindAvailabilityLow  Kind = "availability_low"
	KindNoRun            Kind = "no_run"
	KindTLSExpiry        Kind = "tls_expiry"
)

type State string

const (
	StateOpen         State = "open"
	StateAcknowledged State = "acknowledged"
	StateResolved     State = "resolved"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type EventType string

const (
	EventOpened       EventType = "opened"
	EventFailing      EventType = "failing"
	EventAcknowledged EventType = "acknowledged"
	EventResolved     EventType = "resolved"
	EventReopened     EventType = "reopened"
	EventReminder     EventType = "reminder"
)

const (
	ReopenWindow          = 30 * time.Minute
	ReminderAfter         = 1 * time.Hour
	ReminderSecondAfter   = 24 * time.Hour
	MinOpenAfter          = 1
	MaxOpenAfter          = 20
	MinCloseAfter         = 1
	MaxCloseAfter         = 20
	DefaultOpenAfter      = 3
	DefaultCloseAfter     = 2
	DefaultAvailThreshold = 99.0
	DefaultAvailRecovery  = 99.5
	DefaultNoRunMultiplier = 3
	NoRunSuppressMinCount = 5
	NoRunSuppressMinRatio = 0.5
)

// AllKinds is the stable order for list-policies responses.
var AllKinds = []Kind{
	KindCallFailed,
	KindWorkflowFailed,
	KindCallSlow,
	KindChainBroken,
	KindAvailabilityLow,
	KindNoRun,
	KindTLSExpiry,
}

func ParseKind(raw string) (Kind, error) {
	k := Kind(raw)
	switch k {
	case KindCallFailed, KindWorkflowFailed, KindCallSlow, KindChainBroken,
		KindAvailabilityLow, KindNoRun, KindTLSExpiry:
		return k, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownKind, raw)
	}
}

func ParseSeverity(raw string) (Severity, error) {
	s := Severity(raw)
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return s, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidSeverity, raw)
	}
}

func IsActive(s State) bool {
	return s == StateOpen || s == StateAcknowledged
}
