package contract

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	ResultVersion = 1
	ResultType    = "workflow.result"

	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"

	CallStatusSuccess = "success"
	CallStatusFailed  = "failed"
	CallStatusError   = "error"
	CallStatusSkipped = "skipped"
)

// Result is the single message worker-calls publishes on `workflow.results`
// once a run is over. Secrets are already masked.
type Result struct {
	V          int               `json:"v"`
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	UserID     string            `json:"user_id"`
	WorkflowID string            `json:"workflow_id"`
	RunID      string            `json:"run_id"`
	Trigger    string            `json:"trigger"`
	Status     string            `json:"status"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
	DurationMS int64             `json:"duration_ms"`
	Counts     Counts            `json:"counts"`
	Variables  map[string]string `json:"variables,omitempty"`
	Calls      []CallResult      `json:"calls"`
	EmittedAt  time.Time         `json:"emitted_at"`
}

type Counts struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Error   int `json:"error"`
	Skipped int `json:"skipped"`
}

type CallResult struct {
	CallID             string            `json:"call_id"`
	Name               string            `json:"name"`
	Order              int               `json:"order"`
	Status             string            `json:"status"`
	Reason             string            `json:"reason,omitempty"`
	HTTPStatus         int               `json:"http_status,omitempty"`
	Request            RequestInfo       `json:"request"`
	Response           ResponseInfo      `json:"response"`
	TimingsMS          Timings           `json:"timings_ms"`
	Network            NetworkInfo       `json:"network"`
	TLS                *TLSInfo          `json:"tls,omitempty"`
	Rules              []RuleResult      `json:"rules"`
	ExtractedVariables map[string]string `json:"extracted_variables,omitempty"`
	UsedVariables      []string          `json:"used_variables,omitempty"`
	Error              *CallError        `json:"error,omitempty"`
	BodyRef            string            `json:"body_ref,omitempty"`
	StartedAt          time.Time         `json:"started_at"`
	FinishedAt         time.Time         `json:"finished_at"`
}

type RequestInfo struct {
	Method        string            `json:"method,omitempty"`
	URL           string            `json:"url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Query         map[string]string `json:"query,omitempty"`
	Body          string            `json:"body,omitempty"`
	BodyTruncated bool              `json:"body_truncated,omitempty"`
	Size          int               `json:"size"`
}

type ResponseInfo struct {
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Size        int64             `json:"size"`
	Truncated   bool              `json:"truncated,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Compression string            `json:"compression,omitempty"`
}

type Timings struct {
	DNS      int64 `json:"dns"`
	Connect  int64 `json:"connect"`
	TLS      int64 `json:"tls"`
	TTFB     int64 `json:"ttfb"`
	Transfer int64 `json:"transfer"`
	Total    int64 `json:"total"`
}

type NetworkInfo struct {
	IP        string   `json:"ip,omitempty"`
	IPVersion string   `json:"ip_version,omitempty"`
	Redirects []string `json:"redirects,omitempty"`
}

type TLSInfo struct {
	Host           string           `json:"host"`
	Port           int              `json:"port"`
	Version        string           `json:"version"`
	CipherSuite    string           `json:"cipher_suite"`
	Subject        string           `json:"subject"`
	Issuer         string           `json:"issuer"`
	SerialNumber   string           `json:"serial_number"`
	ExpiresAt      time.Time        `json:"expires_at"`
	ChainExpiresAt time.Time        `json:"chain_expires_at"`
	ChainValid     bool             `json:"chain_valid"`
	NameMatch      bool             `json:"name_match"`
	VerifyError    string           `json:"verify_error,omitempty"`
	Chain          []TLSCertificate `json:"chain,omitempty"`
}

type TLSCertificate struct {
	Subject  string    `json:"subject"`
	Issuer   string    `json:"issuer"`
	NotAfter time.Time `json:"not_after"`
}

type RuleResult struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Text     string `json:"text"`
	Observed string `json:"observed"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
}

const (
	ErrorTypeDNS          = "dns"
	ErrorTypeConnect      = "connect"
	ErrorTypeTLS          = "tls"
	ErrorTypeTimeout      = "timeout"
	ErrorTypeBodyTooLarge = "body_too_large"
	ErrorTypeBlocked      = "blocked"
	ErrorTypeInvalid      = "invalid_request"
	ErrorTypeOther        = "other"
)

type CallError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var ErrInvalidResult = errors.New("invalid result")

func (r Result) Validate() error {
	if r.V != ResultVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidResult, r.V)
	}
	if r.Type != ResultType {
		return fmt.Errorf("%w: unexpected type %q", ErrInvalidResult, r.Type)
	}
	for field, value := range map[string]string{
		"id": r.ID, "user_id": r.UserID, "workflow_id": r.WorkflowID, "run_id": r.RunID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s is not a uuid", ErrInvalidResult, field)
		}
	}
	if r.Status != RunStatusSuccess && r.Status != RunStatusFailed {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidResult, r.Status)
	}
	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() {
		return fmt.Errorf("%w: started_at and finished_at are required", ErrInvalidResult)
	}
	for _, call := range r.Calls {
		switch call.Status {
		case CallStatusSuccess, CallStatusFailed, CallStatusError, CallStatusSkipped:
		default:
			return fmt.Errorf("%w: call %q has unknown status %q", ErrInvalidResult, call.Name, call.Status)
		}
		if call.CallID != "" {
			if _, err := uuid.Parse(call.CallID); err != nil {
				return fmt.Errorf("%w: call %q has an invalid call_id", ErrInvalidResult, call.Name)
			}
		}
	}
	return nil
}
