package presenter

import (
	"time"

	domainincident "go-api/internal/domain/incident"

	"github.com/google/uuid"
)

type IncidentPolicyResponse struct {
	Kind       string         `json:"kind"`
	Enabled    bool           `json:"enabled"`
	Severity   string         `json:"severity"`
	OpenAfter  int            `json:"openAfter"`
	CloseAfter int            `json:"closeAfter"`
	Params     map[string]any `json:"params"`
	UpdatedAt  string         `json:"updatedAt,omitempty"`
}

type IncidentPoliciesResponse struct {
	Policies []IncidentPolicyResponse `json:"policies"`
}

func NewIncidentPoliciesResponse(policies []domainincident.Policy) IncidentPoliciesResponse {
	out := make([]IncidentPolicyResponse, 0, len(policies))
	for _, p := range policies {
		out = append(out, NewIncidentPolicyResponse(p))
	}
	return IncidentPoliciesResponse{Policies: out}
}

func NewIncidentPolicyResponse(p domainincident.Policy) IncidentPolicyResponse {
	params := p.Params
	if params == nil {
		params = map[string]any{}
	}
	resp := IncidentPolicyResponse{
		Kind: string(p.Kind), Enabled: p.Enabled, Severity: string(p.Severity),
		OpenAfter: p.OpenAfter, CloseAfter: p.CloseAfter, Params: params,
	}
	if p.ID != uuid.Nil && !p.UpdatedAt.IsZero() {
		resp.UpdatedAt = p.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

type IncidentListItemResponse struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	CallID      *string `json:"callId,omitempty"`
	Severity    string  `json:"severity"`
	State       string  `json:"state"`
	OpenedAt    string  `json:"openedAt"`
	ResolvedAt  *string `json:"resolvedAt,omitempty"`
	ReopenCount int     `json:"reopenCount"`
}

func NewIncidentListResponse(incidents []domainincident.Incident) []IncidentListItemResponse {
	out := make([]IncidentListItemResponse, 0, len(incidents))
	for _, inc := range incidents {
		out = append(out, newIncidentListItem(inc))
	}
	return out
}

func newIncidentListItem(inc domainincident.Incident) IncidentListItemResponse {
	item := IncidentListItemResponse{
		ID: inc.ID.String(), Kind: string(inc.Kind),
		Severity: string(inc.Severity), State: string(inc.State),
		OpenedAt: inc.OpenedAt.UTC().Format(time.RFC3339),
		ReopenCount: inc.ReopenCount,
	}
	if inc.CallID != nil {
		s := inc.CallID.String()
		item.CallID = &s
	}
	if inc.ResolvedAt != nil {
		s := inc.ResolvedAt.UTC().Format(time.RFC3339)
		item.ResolvedAt = &s
	}
	return item
}

type IncidentEventResponse struct {
	At      string         `json:"at"`
	Type    string         `json:"type"`
	RunID   *string        `json:"runId,omitempty"`
	Payload map[string]any `json:"payload"`
}

type IncidentDetailResponse struct {
	ID             string                  `json:"id"`
	WorkflowID     string                  `json:"workflowId"`
	Kind           string                  `json:"kind"`
	CallID         *string                 `json:"callId,omitempty"`
	Fingerprint    string                  `json:"fingerprint"`
	Severity       string                  `json:"severity"`
	State          string                  `json:"state"`
	Cause          map[string]any          `json:"cause"`
	OpenedAt       string                  `json:"openedAt"`
	AcknowledgedAt *string                 `json:"acknowledgedAt,omitempty"`
	AcknowledgedBy string                  `json:"acknowledgedBy,omitempty"`
	ResolvedAt     *string                 `json:"resolvedAt,omitempty"`
	ReopenCount    int                     `json:"reopenCount"`
	Events         []IncidentEventResponse `json:"events"`
}

func NewIncidentDetailResponse(inc domainincident.Incident, events []domainincident.Event) IncidentDetailResponse {
	cause := inc.Cause
	if cause == nil {
		cause = map[string]any{}
	}
	resp := IncidentDetailResponse{
		ID: inc.ID.String(), WorkflowID: inc.WorkflowID.String(),
		Kind: string(inc.Kind), Fingerprint: inc.Fingerprint,
		Severity: string(inc.Severity), State: string(inc.State),
		Cause: cause, OpenedAt: inc.OpenedAt.UTC().Format(time.RFC3339),
		AcknowledgedBy: inc.AcknowledgedBy, ReopenCount: inc.ReopenCount,
		Events: make([]IncidentEventResponse, 0, len(events)),
	}
	if inc.CallID != nil {
		s := inc.CallID.String()
		resp.CallID = &s
	}
	if inc.AcknowledgedAt != nil {
		s := inc.AcknowledgedAt.UTC().Format(time.RFC3339)
		resp.AcknowledgedAt = &s
	}
	if inc.ResolvedAt != nil {
		s := inc.ResolvedAt.UTC().Format(time.RFC3339)
		resp.ResolvedAt = &s
	}
	for _, e := range events {
		payload := e.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		ev := IncidentEventResponse{
			At: e.At.UTC().Format(time.RFC3339), Type: string(e.Type), Payload: payload,
		}
		if e.RunID != nil {
			s := e.RunID.String()
			ev.RunID = &s
		}
		resp.Events = append(resp.Events, ev)
	}
	return resp
}
