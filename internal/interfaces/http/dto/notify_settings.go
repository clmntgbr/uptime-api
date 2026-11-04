package dto

type UpsertNotificationSettingsRequest struct {
	Emails           []string `json:"emails"`
	IncidentsEnabled bool     `json:"incidentsEnabled"`
	TLSEnabled       bool     `json:"tlsEnabled"`
}
