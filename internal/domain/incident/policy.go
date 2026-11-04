package incident

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Policy struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	WorkflowID uuid.UUID
	Kind       Kind
	Enabled    bool
	Severity   Severity
	OpenAfter  int
	CloseAfter int
	Params     map[string]any
	UpdatedAt  time.Time
}

// DefaultPolicy is the virtual policy when no row exists.
func DefaultPolicy(userID, workflowID uuid.UUID, kind Kind) Policy {
	p := Policy{
		UserID:     userID,
		WorkflowID: workflowID,
		Kind:       kind,
		Enabled:    true,
		Severity:   SeverityCritical,
		OpenAfter:  DefaultOpenAfter,
		CloseAfter: DefaultCloseAfter,
		Params:     map[string]any{},
		UpdatedAt:  time.Now().UTC(),
	}
	switch kind {
	case KindCallFailed, KindWorkflowFailed:
		p.Severity = SeverityCritical
		p.Enabled = true
		p.OpenAfter = DefaultOpenAfter
		p.CloseAfter = DefaultCloseAfter
	case KindCallSlow:
		p.Severity = SeverityWarning
		p.Enabled = true
	case KindChainBroken:
		p.Severity = SeverityWarning
		p.Enabled = true
		p.OpenAfter = 2
		p.CloseAfter = DefaultCloseAfter
	case KindAvailabilityLow:
		p.Severity = SeverityWarning
		p.Enabled = false
		p.OpenAfter = 1
		p.CloseAfter = DefaultCloseAfter
		p.Params = map[string]any{
			"threshold":         DefaultAvailThreshold,
			"recoveryThreshold": DefaultAvailRecovery,
		}
	case KindNoRun:
		p.Severity = SeverityCritical
		p.Enabled = true
		p.OpenAfter = 1
		p.CloseAfter = 1
		p.Params = map[string]any{"multiplier": DefaultNoRunMultiplier}
	case KindTLSExpiry:
		p.Severity = SeverityWarning
		p.Enabled = true
		p.OpenAfter = 1
		p.CloseAfter = 1
	}
	return p
}

func ValidatePolicyFields(enabled bool, severity Severity, openAfter, closeAfter int, kind Kind, params map[string]any) error {
	if openAfter < MinOpenAfter || openAfter > MaxOpenAfter {
		return fmt.Errorf("%w: openAfter must be between %d and %d", ErrInvalidPolicy, MinOpenAfter, MaxOpenAfter)
	}
	if closeAfter < MinCloseAfter || closeAfter > MaxCloseAfter {
		return fmt.Errorf("%w: closeAfter must be between %d and %d", ErrInvalidPolicy, MinCloseAfter, MaxCloseAfter)
	}
	if _, err := ParseSeverity(string(severity)); err != nil {
		return err
	}
	_ = enabled
	if kind == KindAvailabilityLow {
		th := floatFromParams(params, "threshold", DefaultAvailThreshold)
		rec := floatFromParams(params, "recoveryThreshold", DefaultAvailRecovery)
		if rec < th {
			return fmt.Errorf("%w: recoveryThreshold must be >= threshold", ErrInvalidPolicy)
		}
	}
	return nil
}

func floatFromParams(params map[string]any, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	v, ok := params[key]
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return fallback
	}
}

func (p Policy) AvailabilityThresholds() (threshold, recovery float64) {
	return floatFromParams(p.Params, "threshold", DefaultAvailThreshold),
		floatFromParams(p.Params, "recoveryThreshold", DefaultAvailRecovery)
}

func (p Policy) NoRunMultiplier() int {
	if p.Params == nil {
		return DefaultNoRunMultiplier
	}
	v, ok := p.Params["multiplier"]
	if !ok || v == nil {
		return DefaultNoRunMultiplier
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return DefaultNoRunMultiplier
	}
}
