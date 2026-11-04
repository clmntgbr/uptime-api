package presenter

import (
	"time"

	domainnotify "go-api/internal/domain/notifysettings"
)

type NotificationSettingsResponse struct {
	Emails           []string `json:"emails"`
	IncidentsEnabled bool     `json:"incidentsEnabled"`
	TLSEnabled       bool     `json:"tlsEnabled"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

func NewNotificationSettingsResponse(s domainnotify.Settings) NotificationSettingsResponse {
	emails := s.Emails
	if emails == nil {
		emails = []string{}
	}
	resp := NotificationSettingsResponse{
		Emails: emails, IncidentsEnabled: s.IncidentsEnabled,
		TLSEnabled: s.TLSEnabled,
	}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
