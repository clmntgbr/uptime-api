package notifysettings

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxEmails = 20

type RunMode string

const (
	RunModeEvery      RunMode = "every"
	RunModeTransition RunMode = "transition"
)

type Settings struct {
	WorkflowID       uuid.UUID
	UserID           uuid.UUID
	Emails           []string
	IncidentsEnabled bool
	TLSEnabled       bool
	RunsEnabled      bool
	RunStatuses      []string
	RunMode          RunMode
	UpdatedAt        time.Time
}

// DefaultSettings is the virtual policy when no row exists yet.
func DefaultSettings(userID, workflowID uuid.UUID, creatorEmail string, now time.Time) Settings {
	emails := NormalizeEmails([]string{creatorEmail})
	return Settings{
		WorkflowID:       workflowID,
		UserID:           userID,
		Emails:           emails,
		IncidentsEnabled: true,
		TLSEnabled:       true,
		RunsEnabled:      false,
		RunStatuses:      []string{"failed", "error"},
		RunMode:          RunModeTransition,
		UpdatedAt:        now,
	}
}

func NormalizeEmails(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		e := strings.ToLower(strings.TrimSpace(raw))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func ValidateEmails(emails []string) error {
	if len(emails) > MaxEmails {
		return fmt.Errorf("%w: at most %d emails", ErrInvalid, MaxEmails)
	}
	for _, e := range emails {
		if _, err := mail.ParseAddress(e); err != nil {
			return fmt.Errorf("%w: invalid email %q", ErrInvalid, e)
		}
	}
	return nil
}

func NormalizeRunStatuses(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		s := strings.ToLower(strings.TrimSpace(raw))
		switch s {
		case "failed", "error", "success", "skipped":
		default:
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return []string{"failed", "error"}
	}
	return out
}

func ParseRunMode(raw string) (RunMode, error) {
	switch RunMode(raw) {
	case RunModeEvery, RunModeTransition, "":
		if raw == "" {
			return RunModeTransition, nil
		}
		return RunMode(raw), nil
	default:
		return "", fmt.Errorf("%w: invalid run mode %q", ErrInvalid, raw)
	}
}

func (s Settings) AllowsIncident() bool {
	return s.IncidentsEnabled && len(s.Emails) > 0
}

func (s Settings) AllowsTLS() bool {
	return s.TLSEnabled && len(s.Emails) > 0
}

func (s Settings) AllowsRuns() bool {
	return s.RunsEnabled && len(s.Emails) > 0
}

// ShouldNotifyRun reports whether this run outcome should trigger a mail.
func (s Settings) ShouldNotifyRun(status, previousStatus string) bool {
	if !s.AllowsRuns() {
		return false
	}
	watched := false
	for _, st := range s.RunStatuses {
		if st == status {
			watched = true
			break
		}
	}
	if !watched {
		return false
	}
	if s.RunMode == RunModeEvery {
		return true
	}
	return status != previousStatus
}
