package notifysettings

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxEmails = 20

type Settings struct {
	WorkflowID       uuid.UUID
	UserID           uuid.UUID
	Emails           []string
	IncidentsEnabled bool
	TLSEnabled       bool
	UpdatedAt        time.Time
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

func (s Settings) AllowsIncident() bool {
	return s.IncidentsEnabled && len(s.Emails) > 0
}

func (s Settings) AllowsTLS() bool {
	return s.TLSEnabled && len(s.Emails) > 0
}
