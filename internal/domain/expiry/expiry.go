package expiry

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeTLSCertificate Type = "tls_certificate"

	MaxThresholds     = 20
	MinThresholdDays  = 1
	MaxThresholdDays  = 365
	StaleAfter       = 7 * 24 * time.Hour
	ExpiredAlertDays = 7
)

type ItemState string

const (
	StateOK      ItemState = "ok"
	StateWarning ItemState = "warning"
	StateExpired ItemState = "expired"
	StateStale   ItemState = "stale"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Policy struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	WorkflowID     uuid.UUID
	Type           Type
	Enabled        bool
	ThresholdsDays []int
	UpdatedAt      time.Time
}

// DefaultPolicy is the virtual policy when the workflow has no row yet:
// alerts off, no thresholds — configure at create or via PUT.
func DefaultPolicy(userID, workflowID uuid.UUID, t Type) Policy {
	return Policy{
		UserID:         userID,
		WorkflowID:     workflowID,
		Type:           t,
		Enabled:        false,
		ThresholdsDays: []int{},
		UpdatedAt:      time.Now().UTC(),
	}
}

// NormalizePolicyThresholds allows empty thresholds only when disabled.
func NormalizePolicyThresholds(enabled bool, in []int) ([]int, error) {
	if !enabled && len(in) == 0 {
		return []int{}, nil
	}
	return NormalizeThresholds(in)
}

type Item struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	WorkflowID  uuid.UUID
	Type        Type
	Subject     string
	ExpiresAt   time.Time
	Fingerprint string
	Meta        map[string]string
	ObservedAt  time.Time
}

type Observation struct {
	Type        Type
	Subject     string
	ExpiresAt   time.Time
	Fingerprint string
	Meta        map[string]string
	ObservedAt  time.Time
}

// NotificationRequest is published to notifications.requests after thresholds fire.
type NotificationRequest struct {
	Kind           string    `json:"kind"`
	V              int       `json:"v"`
	Type           Type      `json:"type"`
	UserID         uuid.UUID `json:"user_id"`
	WorkflowID     uuid.UUID `json:"workflow_id"`
	ItemID         uuid.UUID `json:"item_id"`
	Subject        string    `json:"subject"`
	DaysLeft       int       `json:"days_left"`
	ThresholdDays  int       `json:"threshold_days"`
	ExpiresAt      time.Time `json:"expires_at"`
	Severity       Severity  `json:"severity"`
	Message        string    `json:"message"`
}

type AlertClaim struct {
	ThresholdDays int
	Fingerprint   string
}

// NormalizeThresholds dedupes, validates, and sorts descending.
func NormalizeThresholds(in []int) ([]int, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("%w: at least one threshold is required when enabled", ErrInvalidThresholds)
	}
	if len(in) > MaxThresholds {
		return nil, fmt.Errorf("%w: at most %d thresholds", ErrInvalidThresholds, MaxThresholds)
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, d := range in {
		if d < MinThresholdDays || d > MaxThresholdDays {
			return nil, fmt.Errorf("%w: threshold %d must be between %d and %d", ErrInvalidThresholds, d, MinThresholdDays, MaxThresholdDays)
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out, nil
}

func ParseType(raw string) (Type, error) {
	switch Type(raw) {
	case TypeTLSCertificate:
		return TypeTLSCertificate, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownType, raw)
	}
}

// DaysLeft is floor((expiresAt - now) / 24h).
func DaysLeft(expiresAt, now time.Time) int {
	d := expiresAt.UTC().Sub(now.UTC())
	return int(d / (24 * time.Hour))
}

func SeverityFor(daysLeft int) Severity {
	if daysLeft < 4 {
		return SeverityCritical
	}
	if daysLeft <= 14 {
		return SeverityWarning
	}
	return SeverityInfo
}

func MessageFor(t Type, subject string, daysLeft int) string {
	switch t {
	case TypeTLSCertificate:
		if daysLeft < 0 {
			return fmt.Sprintf("The certificate of %s has expired", subject)
		}
		if daysLeft == 0 {
			return fmt.Sprintf("The certificate of %s expires today", subject)
		}
		return fmt.Sprintf("The certificate of %s expires in %d days", subject, daysLeft)
	default:
		return fmt.Sprintf("%s (%s) expires in %d days", t, subject, daysLeft)
	}
}

// CrossedThresholds returns policy thresholds where daysLeft <= threshold.
func CrossedThresholds(thresholds []int, daysLeft int) []int {
	var out []int
	for _, t := range thresholds {
		if daysLeft <= t {
			out = append(out, t)
		}
	}
	return out
}

// AlertFingerprint keys a claim. Expired items use fingerprint|date for daily re-alerts.
func AlertFingerprint(itemFingerprint string, daysLeft int, now time.Time) string {
	if daysLeft < 0 {
		return itemFingerprint + "|" + now.UTC().Format("2006-01-02")
	}
	return itemFingerprint
}

// NotifyThreshold is the smallest crossed threshold (most urgent), or 0 when expired.
func NotifyThreshold(crossed []int, daysLeft int) int {
	if daysLeft < 0 {
		return 0
	}
	if len(crossed) == 0 {
		return 0
	}
	min := crossed[0]
	for _, t := range crossed[1:] {
		if t < min {
			min = t
		}
	}
	return min
}

func ItemViewState(item Item, thresholds []int, now time.Time) (ItemState, int, *int) {
	if now.Sub(item.ObservedAt) > StaleAfter {
		days := DaysLeft(item.ExpiresAt, now)
		return StateStale, days, nextThreshold(thresholds, days)
	}
	days := DaysLeft(item.ExpiresAt, now)
	if days < 0 {
		return StateExpired, days, nil
	}
	for _, t := range thresholds {
		if days <= t {
			nt := t
			return StateWarning, days, &nt
		}
	}
	return StateOK, days, nextThreshold(thresholds, days)
}

// nextThreshold is the closest threshold still ahead (max t with daysLeft > t).
func nextThreshold(thresholds []int, daysLeft int) *int {
	if daysLeft <= 0 || len(thresholds) == 0 {
		return nil
	}
	var best *int
	for _, t := range thresholds {
		if daysLeft > t {
			if best == nil || t > *best {
				v := t
				best = &v
			}
		}
	}
	return best
}

// SubjectHostPort builds host:port for TLS observations.
func SubjectHostPort(host string, port int) string {
	if port <= 0 {
		port = 443
	}
	return fmt.Sprintf("%s:%d", host, port)
}
