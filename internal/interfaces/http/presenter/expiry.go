package presenter

import (
	"time"

	domainexpiry "go-api/internal/domain/expiry"

	"github.com/google/uuid"
)

type ExpiryPolicyResponse struct {
	Type           string `json:"type"`
	Enabled        bool   `json:"enabled"`
	ThresholdsDays []int  `json:"thresholdsDays"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type ExpiryPoliciesResponse struct {
	Policies []ExpiryPolicyResponse `json:"policies"`
}

func NewExpiryPoliciesResponse(policies []domainexpiry.Policy) ExpiryPoliciesResponse {
	out := make([]ExpiryPolicyResponse, 0, len(policies))
	for _, p := range policies {
		out = append(out, NewExpiryPolicyResponse(p))
	}
	return ExpiryPoliciesResponse{Policies: out}
}

func NewExpiryPolicyResponse(p domainexpiry.Policy) ExpiryPolicyResponse {
	days := append([]int(nil), p.ThresholdsDays...)
	resp := ExpiryPolicyResponse{
		Type: string(p.Type), Enabled: p.Enabled, ThresholdsDays: days,
	}
	if p.ID != uuid.Nil && !p.UpdatedAt.IsZero() {
		resp.UpdatedAt = p.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

type ExpiryItemsResponse struct {
	Items []ExpiryItemResponse `json:"items"`
}

type ExpiryItemResponse struct {
	ID                string  `json:"id"`
	Type              string  `json:"type"`
	Subject           string  `json:"subject"`
	ExpiresAt         string  `json:"expiresAt"`
	DaysLeft          int     `json:"daysLeft"`
	State             string  `json:"state"`
	NextThresholdDays *int    `json:"nextThresholdDays,omitempty"`
	Issuer            string  `json:"issuer,omitempty"`
	ObservedAt        string  `json:"observedAt"`
}

func NewExpiryItemsResponse(views []domainexpiry.ItemView) ExpiryItemsResponse {
	out := make([]ExpiryItemResponse, 0, len(views))
	for _, v := range views {
		out = append(out, ExpiryItemResponse{
			ID:                v.ID.String(),
			Type:              string(v.Type),
			Subject:           v.Subject,
			ExpiresAt:         v.ExpiresAt.UTC().Format(time.RFC3339),
			DaysLeft:          v.DaysLeft,
			State:             string(v.State),
			NextThresholdDays: v.NextThresholdDays,
			Issuer:            v.Issuer,
			ObservedAt:        v.ObservedAt.UTC().Format(time.RFC3339),
		})
	}
	return ExpiryItemsResponse{Items: out}
}
