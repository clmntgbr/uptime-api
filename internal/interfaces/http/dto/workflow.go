package dto

// WorkflowRequest is the body of POST /workflows and PUT /workflows/:id. A PUT
// replaces the whole definition; calls keep their identity by name.
type WorkflowRequest struct {
	Name            string                  `json:"name" validate:"required,max=200"`
	IntervalSeconds int                     `json:"intervalSeconds" validate:"required,min=60,max=604800"`
	Enabled         *bool                   `json:"enabled"`
	Statics         []WorkflowStaticRequest `json:"statics" validate:"max=50,dive"`
	Calls           []WorkflowCallRequest   `json:"calls" validate:"required,min=1,max=20,dive"`
}

type WorkflowStaticRequest struct {
	Name  string `json:"name" validate:"required,max=64"`
	Value string `json:"value" validate:"max=4096"`
}

type WorkflowCallRequest struct {
	Name        string                      `json:"name" validate:"required,max=100"`
	Method      string                      `json:"method" validate:"omitempty,oneof=GET POST PUT PATCH DELETE HEAD OPTIONS"`
	URL         string                      `json:"url" validate:"required,max=2048"`
	Headers     map[string]string           `json:"headers" validate:"max=50"`
	Query       map[string]string           `json:"query" validate:"max=50"`
	Body        string                      `json:"body" validate:"max=65536"`
	TimeoutMS   int                         `json:"timeoutMs" validate:"omitempty,min=100,max=60000"`
	VerifyTLS   *bool                       `json:"verifyTls"`
	IPVersion   string                      `json:"ipVersion" validate:"omitempty,oneof=any ipv4 ipv6"`
	ProxyURL    string                      `json:"proxyUrl" validate:"max=2048"`
	Extractions []WorkflowExtractionRequest `json:"extractions" validate:"max=20,dive"`
	Rules       []WorkflowRuleRequest       `json:"rules" validate:"max=20,dive"`
}

type WorkflowExtractionRequest struct {
	Name       string `json:"name" validate:"required,max=64"`
	Source     string `json:"source" validate:"required,oneof=json header status regex"`
	Expression string `json:"expression" validate:"max=512"`
}

// WorkflowRuleRequest is one rule sentence: subject, operator and value, plus
// the JSONPath or header name for json and header rules.
type WorkflowRuleRequest struct {
	Subject  string `json:"subject" validate:"required,oneof=status response_time body json header size certificate"`
	Operator string `json:"operator" validate:"required,oneof=is is_not in exceeds below contains not_contains exists not_exists expires_within chain_invalid name_mismatch"`
	Target   string `json:"target" validate:"max=512"`
	Value    string `json:"value" validate:"max=1024"`
}
