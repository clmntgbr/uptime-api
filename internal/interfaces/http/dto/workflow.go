package dto

import "time"

// WorkflowRequest is the body of POST /workflows and PUT /workflows/:id.
// Calls and static variables are managed by dedicated endpoints.
// ExpiryPolicies is honored on create only (ignored on update).
type WorkflowRequest struct {
	Name            string                    `json:"name" validate:"required,max=200"`
	ScheduleType    string                    `json:"scheduleType" validate:"omitempty,oneof=interval once at"`
	IntervalSeconds *int                      `json:"intervalSeconds" validate:"omitempty,min=60,max=604800"`
	RunAt           *time.Time                `json:"runAt"`
	Enabled         *bool                     `json:"enabled"`
	ExpiryPolicies  []WorkflowExpiryPolicyInput `json:"expiryPolicies" validate:"omitempty,dive"`
}

// WorkflowExpiryPolicyInput configures an expiry type at workflow creation.
type WorkflowExpiryPolicyInput struct {
	Type           string `json:"type" validate:"required,oneof=tls_certificate"`
	Enabled        bool   `json:"enabled"`
	ThresholdsDays []int  `json:"thresholdsDays" validate:"omitempty,max=20,dive,min=1,max=365"`
}
