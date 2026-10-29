package dto

// UpsertExpiryPolicyRequest is PUT /workflows/:id/expiry-policies/:type.
type UpsertExpiryPolicyRequest struct {
	Enabled        bool  `json:"enabled"`
	ThresholdsDays []int `json:"thresholdsDays" validate:"omitempty,max=20,dive,min=1,max=365"`
}
