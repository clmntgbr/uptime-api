package presenter

import (
	"time"

	domainnotify "go-api/internal/domain/notifysettings"
)

type NotificationSettingsResponse struct {
	Emails           []string `json:"emails"`
	IncidentsEnabled bool     `json:"incidentsEnabled"`
	TLSEnabled       bool     `json:"tlsEnabled"`
	RunsEnabled      bool     `json:"runsEnabled"`
	RunStatuses      []string `json:"runStatuses"`
	RunMode          string   `json:"runMode"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

func NewNotificationSettingsResponse(s domainnotify.Settings) NotificationSettingsResponse {
	emails := s.Emails
	if emails == nil {
		emails = []string{}
	}
	statuses := s.RunStatuses
	if statuses == nil {
		statuses = []string{}
	}
	mode := string(s.RunMode)
	if mode == "" {
		mode = string(domainnotify.RunModeTransition)
	}
	resp := NotificationSettingsResponse{
		Emails: emails, IncidentsEnabled: s.IncidentsEnabled,
		TLSEnabled: s.TLSEnabled, RunsEnabled: s.RunsEnabled,
		RunStatuses: statuses, RunMode: mode,
	}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
