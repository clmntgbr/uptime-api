package dto

import "time"

// WorkflowRequest is the body of POST /workflows and PUT /workflows/:id.
// Calls and static variables are managed by dedicated endpoints.
type WorkflowRequest struct {
	Name            string     `json:"name" validate:"required,max=200"`
	ScheduleType    string     `json:"scheduleType" validate:"omitempty,oneof=interval once at"`
	IntervalSeconds *int       `json:"intervalSeconds" validate:"omitempty,min=60,max=604800"`
	RunAt           *time.Time `json:"runAt"`
	Enabled         *bool      `json:"enabled"`
}
