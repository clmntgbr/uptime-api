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

	CallProgressVersion = 1
	CallProgressType    = "workflow.call_progress"

	CallProgressPhaseStarted  = "started"
	CallProgressPhaseFinished = "finished"

	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
	RunStatusRunning = "running"

	CallStatusSuccess = "success"
	CallStatusFailed  = "failed"
	CallStatusError   = "error"
	CallStatusSkipped = "skipped"
)

// CallProgress is published on `workflow.results` when a call starts or finishes,
// so the main worker can persist call_runs progressively and push realtime events.
// Secrets are masked.
type CallProgress struct {
	V          int       `json:"v"`
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Phase      string    `json:"phase"`
	UserID     string    `json:"user_id"`
	WorkflowID string    `json:"workflow_id"`
	RunID      string    `json:"run_id"`
	Trigger    string    `json:"trigger"`
	Call       CallRun   `json:"call"`
	EmittedAt  time.Time `json:"emitted_at"`
}

func (p CallProgress) Validate() error {
	if p.V != CallProgressVersion {
		return fmt.Errorf("%w: unsupported call progress version %d", ErrInvalidResult, p.V)
	}
	if p.Type != CallProgressType {
		return fmt.Errorf("%w: unexpected type %q", ErrInvalidResult, p.Type)
	}
	phase := p.Phase
	if phase == "" {
		phase = CallProgressPhaseFinished
	}
	switch phase {
	case CallProgressPhaseStarted, CallProgressPhaseFinished:
	default:
		return fmt.Errorf("%w: unknown phase %q", ErrInvalidResult, p.Phase)
	}
	for field, value := range map[string]string{
		"id": p.ID, "user_id": p.UserID, "workflow_id": p.WorkflowID, "run_id": p.RunID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s is not a uuid", ErrInvalidResult, field)
		}
	}
	if p.Call.Name == "" || p.Call.Order < 1 {
		return fmt.Errorf("%w: call name and order are required", ErrInvalidResult)
	}
	if p.Call.CallID != "" {
		if _, err := uuid.Parse(p.Call.CallID); err != nil {
			return fmt.Errorf("%w: call %q has an invalid call_id", ErrInvalidResult, p.Call.Name)
		}
	}
	if phase == CallProgressPhaseFinished {
		switch p.Call.Status {
		case CallStatusSuccess, CallStatusFailed, CallStatusError, CallStatusSkipped:
		default:
			return fmt.Errorf("%w: call %q has unknown status %q", ErrInvalidResult, p.Call.Name, p.Call.Status)
		}
	}
	return nil
}

// NormalizedPhase returns the progress phase, defaulting empty to finished.
func (p CallProgress) NormalizedPhase() string {
	if p.Phase == "" {
		return CallProgressPhaseFinished
	}
	return p.Phase
}

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
	Calls      []CallRun         `json:"calls"`
	EmittedAt  time.Time         `json:"emitted_at"`
}

type Counts struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Error   int `json:"error"`
	Skipped int `json:"skipped"`
}

type CallRun struct {
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
	Rules              []RuleRun         `json:"rules"`
	ExtractedVariables map[string]string `json:"extracted_variables,omitempty"`
	UsedVariables      []string          `json:"used_variables,omitempty"`
	Error              *CallError        `json:"error,omitempty"`
	// BodyRef is the claim-check object key (s3://bucket/…). Empty when the
	// full body fits in Response.Body.
	BodyRef string `json:"body_ref,omitempty"`
	// BodyHash is the SHA-256 hex of the full masked body when offloaded.
	BodyHash string `json:"body_hash,omitempty"`
	// BodyEncoding is the compression used in the object store (zstd).
	BodyEncoding string    `json:"body_encoding,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
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
	Headers map[string]string `json:"headers,omitempty"`
	// Body is the full plaintext body when small, otherwise a short preview.
	Body string `json:"body,omitempty"`
	Size int64  `json:"size"`
	// Truncated is true when Body is only a preview (full body in object store
	// or preview-only after an upload failure).
	Truncated bool   `json:"truncated,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	// Compression is the Content-Encoding observed on the wire (e.g. gzip).
	Compression string `json:"compression,omitempty"`
	// Keys are JSON paths found in the body, for later path search.
	Keys []string `json:"keys,omitempty"`
}

const BodyEncodingZstd = "zstd"

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

type RuleRun struct {
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
