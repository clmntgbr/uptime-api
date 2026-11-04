package dto

// UpsertIncidentPolicyRequest is PUT /workflows/:id/incident-policies/:kind.
type UpsertIncidentPolicyRequest struct {
	Enabled    bool           `json:"enabled"`
	Severity   string         `json:"severity" validate:"required,oneof=info warning critical"`
	OpenAfter  int            `json:"openAfter" validate:"required,min=1,max=20"`
	CloseAfter int            `json:"closeAfter" validate:"required,min=1,max=20"`
	Params     map[string]any `json:"params"`
}
