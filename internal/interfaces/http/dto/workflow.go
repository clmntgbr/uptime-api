package dto

// WorkflowRequest is the body of POST /workflows and PUT /workflows/:id.
// Calls and static variables are managed by dedicated endpoints.
type WorkflowRequest struct {
	Name            string `json:"name" validate:"required,max=200"`
	IntervalSeconds int    `json:"intervalSeconds" validate:"required,min=60,max=604800"`
	Enabled         *bool  `json:"enabled"`
}
