package dto

type UpsertNotificationSettingsRequest struct {
	Emails           []string `json:"emails"`
	IncidentsEnabled bool     `json:"incidentsEnabled"`
	TLSEnabled       bool     `json:"tlsEnabled"`
	RunsEnabled      bool     `json:"runsEnabled"`
	RunStatuses       []string `json:"runStatuses"`
	RunMode          string   `json:"runMode" validate:"omitempty,oneof=every transition"`
}
