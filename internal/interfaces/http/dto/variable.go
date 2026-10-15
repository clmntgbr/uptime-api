package dto

// GlobalVariableRequest is POST /variables.
type GlobalVariableRequest struct {
	Name   string `json:"name" validate:"required,max=64"`
	Value  string `json:"value" validate:"required,max=4096"`
	Secret bool   `json:"secret"`
}

// GlobalVariableUpdateRequest is PUT /variables/:id.
type GlobalVariableUpdateRequest struct {
	Value  string `json:"value" validate:"required,max=4096"`
	Secret bool   `json:"secret"`
}

// StaticVariableRequest is POST /workflows/:id/variables.
type StaticVariableRequest struct {
	Name   string `json:"name" validate:"required,max=64"`
	Value  string `json:"value" validate:"max=4096"`
	Secret bool   `json:"secret"`
}

// StaticVariableUpdateRequest is PUT /workflows/:id/variables/:name.
type StaticVariableUpdateRequest struct {
	Value  string `json:"value" validate:"max=4096"`
	Secret bool   `json:"secret"`
}

// CallVariableRequest is POST /workflows/:id/calls/:callId/variables.
// Path is the extraction expression (JSONPath, header name, regex). Optional:
// omitted with no source → status extraction.
type CallVariableRequest struct {
	Name   string `json:"name" validate:"required,max=64"`
	Path   string `json:"path" validate:"max=512"`
	Source string `json:"source" validate:"omitempty,oneof=json header status regex"`
	Secret bool   `json:"secret"`
}

// CallVariableUpdateRequest is PUT /workflows/:id/calls/:callId/variables/:name.
type CallVariableUpdateRequest struct {
	Path   string `json:"path" validate:"max=512"`
	Source string `json:"source" validate:"omitempty,oneof=json header status regex"`
	Secret bool   `json:"secret"`
}
