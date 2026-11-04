package incident

import (
	"time"
)

// TLSSignal returns a tls_expiry signal for one observation, or nil when the
// observation sits between the last threshold and a higher one (notification-only).
// lastThreshold is the smallest configured threshold (most urgent); 0 means none.
func TLSSignal(subject string, daysLeft int, thresholds []int, observedAt time.Time) *Signal {
	last, maxT := 0, 0
	for _, t := range thresholds {
		if last == 0 || t < last {
			last = t
		}
		if t > maxT {
			maxT = t
		}
	}
	obs := observedAt.UTC()
	switch {
	case daysLeft < 0:
		sev := SeverityCritical
		return &Signal{
			Kind: KindTLSExpiry, Failing: true, Subject: subject, ObservedAt: obs,
			Severity: &sev,
			Cause:    map[string]any{"daysLeft": daysLeft, "expired": true},
		}
	case last > 0 && daysLeft <= last:
		sev := SeverityWarning
		return &Signal{
			Kind: KindTLSExpiry, Failing: true, Subject: subject, ObservedAt: obs,
			Severity: &sev,
			Cause:    map[string]any{"daysLeft": daysLeft, "lastThreshold": last},
		}
	case last == 0 || daysLeft > maxT:
		return &Signal{
			Kind: KindTLSExpiry, Failing: false, Subject: subject, ObservedAt: obs,
			Cause: map[string]any{"daysLeft": daysLeft},
		}
	default:
		// Between last and a higher threshold: expiry notifications only.
		return nil
	}
}

// AvailabilitySignal applies hysteresis: failing below threshold, healthy at or
// above recovery, nil in between.
func AvailabilitySignal(ratio, threshold, recovery float64, observedAt time.Time) *Signal {
	obs := observedAt.UTC()
	cause := map[string]any{"ratio": ratio, "threshold": threshold, "recoveryThreshold": recovery}
	if ratio < threshold {
		return &Signal{Kind: KindAvailabilityLow, Failing: true, ObservedAt: obs, Cause: cause}
	}
	if ratio >= recovery {
		return &Signal{Kind: KindAvailabilityLow, Failing: false, ObservedAt: obs, Cause: cause}
	}
	return nil
}

// ShouldSuppressNoRun is the platform-outage guard: at least minCount silent
// workflows and they are at least minRatio of the enabled set.
func ShouldSuppressNoRun(silent, enabled int, minCount int, minRatio float64) bool {
	if enabled <= 0 || silent <= 0 {
		return false
	}
	if silent < minCount {
		return false
	}
	return float64(silent)/float64(enabled) >= minRatio
}
