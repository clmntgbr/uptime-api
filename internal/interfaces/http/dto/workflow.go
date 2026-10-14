package dto

// WorkflowRequest is the body of POST /workflows and PUT /workflows/:id.
// Calls are managed by dedicated endpoints; this only updates metadata.
type WorkflowRequest struct {
	Name            string                  `json:"name" validate:"required,max=200"`
	IntervalSeconds int                     `json:"intervalSeconds" validate:"required,min=60,max=604800"`
	Enabled         *bool                   `json:"enabled"`
	Statics         []WorkflowStaticRequest `json:"statics" validate:"max=50,dive"`
}

type WorkflowStaticRequest struct {
	Name  string `json:"name" validate:"required,max=64"`
	Value string `json:"value" validate:"max=4096"`
}
