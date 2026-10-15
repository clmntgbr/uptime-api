package dto

// CallRequest is the body of POST /workflows/:id/calls and PUT /workflows/:id/calls/:callId.
type CallRequest struct {
	Name        string                  `json:"name" validate:"required,max=100"`
	Method      string                  `json:"method" validate:"omitempty,oneof=GET POST PUT PATCH DELETE HEAD OPTIONS"`
	URL         string                  `json:"url" validate:"required,max=2048"`
	Headers     map[string]string       `json:"headers" validate:"max=50"`
	Query       map[string]string       `json:"query" validate:"max=50"`
	Body        string                  `json:"body" validate:"max=65536"`
	TimeoutMS   int                     `json:"timeoutMs" validate:"omitempty,min=100,max=60000"`
	VerifyTLS   *bool                   `json:"verifyTls"`
	IPVersion   string                  `json:"ipVersion" validate:"omitempty,oneof=any ipv4 ipv6"`
	ProxyURL    string                  `json:"proxyUrl" validate:"max=2048"`
	Extractions []CallExtractionRequest `json:"extractions" validate:"max=20,dive"`
	Rules       []CallRuleRequest       `json:"rules" validate:"max=20,dive"`
}

type CallExtractionRequest struct {
	Name       string `json:"name" validate:"required,max=64"`
	Source     string `json:"source" validate:"required,oneof=json header status regex"`
	Expression string `json:"expression" validate:"max=512"`
	Secret     bool   `json:"secret"`
}

type CallRuleRequest struct {
	Subject  string `json:"subject" validate:"required,oneof=status response_time body json header size certificate"`
	Operator string `json:"operator" validate:"required,oneof=is is_not in exceeds below contains not_contains exists not_exists expires_within chain_invalid name_mismatch"`
	Target   string `json:"target" validate:"max=512"`
	Value    string `json:"value" validate:"max=1024"`
}
