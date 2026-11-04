package notifysettings

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RunMode string

const (
	RunModeEvery      RunMode = "every"
	RunModeTransition RunMode = "transition"
	MaxEmails         = 20
)

type RunStatus string

const (
	RunSuccess RunStatus = "success"
	RunFailed  RunStatus = "failed"
	RunError   RunStatus = "error"
	RunSkipped RunStatus = "skipped"
)

type Settings struct {
	WorkflowID        uuid.UUID
	UserID            uuid.UUID
	Emails            []string
	IncidentsEnabled  bool
	TLSEnabled        bool
	RunsEnabled       bool
	RunStatuses       []RunStatus
	RunMode           RunMode
	UpdatedAt         time.Time
}

// DefaultSettings is the virtual policy when no row exists yet.
func DefaultSettings(userID, workflowID uuid.UUID, creatorEmail string) Settings {
	emails := NormalizeEmails([]string{creatorEmail})
	return Settings{
		WorkflowID:       workflowID,
		UserID:           userID,
		Emails:           emails,
		IncidentsEnabled: true,
		TLSEnabled:       true,
		RunsEnabled:      false,
		RunStatuses:      nil,
		RunMode:          RunModeTransition,
		UpdatedAt:        time.Now().UTC(),
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

func ParseRunMode(raw string) (RunMode, error) {
	m := RunMode(raw)
	switch m {
	case RunModeEvery, RunModeTransition:
		return m, nil
	default:
		return "", fmt.Errorf("%w: runMode must be every or transition", ErrInvalid)
	}
}

func ParseRunStatuses(raw []string) ([]RunStatus, error) {
	out := make([]RunStatus, 0, len(raw))
	seen := map[RunStatus]struct{}{}
	for _, r := range raw {
		s := RunStatus(strings.TrimSpace(r))
		switch s {
		case RunSuccess, RunFailed, RunError, RunSkipped:
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		default:
			return nil, fmt.Errorf("%w: unknown run status %q", ErrInvalid, r)
		}
	}
	return out, nil
}

func (s Settings) AllowsIncident() bool {
	return s.IncidentsEnabled && len(s.Emails) > 0
}

func (s Settings) AllowsTLS() bool {
	return s.TLSEnabled && len(s.Emails) > 0
}

func (s Settings) AllowsRun(status string, previousStatus string) bool {
	if !s.RunsEnabled || len(s.Emails) == 0 {
		return false
	}
	wanted := false
	for _, st := range s.RunStatuses {
		if string(st) == status {
			wanted = true
			break
		}
	}
	if !wanted {
		return false
	}
	if s.RunMode == RunModeTransition {
		return previousStatus != "" && previousStatus != status
	}
	return true
}
