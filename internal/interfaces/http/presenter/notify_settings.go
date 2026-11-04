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
	RunStatuses       []string `json:"runStatuses"`
	RunMode          string   `json:"runMode"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

func NewNotificationSettingsResponse(s domainnotify.Settings) NotificationSettingsResponse {
	statuses := make([]string, 0, len(s.RunStatuses))
	for _, st := range s.RunStatuses {
		statuses = append(statuses, string(st))
	}
	emails := s.Emails
	if emails == nil {
		emails = []string{}
	}
	resp := NotificationSettingsResponse{
		Emails: emails, IncidentsEnabled: s.IncidentsEnabled,
		TLSEnabled: s.TLSEnabled, RunsEnabled: s.RunsEnabled,
		RunStatuses: statuses, RunMode: string(s.RunMode),
	}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
