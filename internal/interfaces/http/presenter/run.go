package presenter

import (
	"time"

	domainrun "go-api/internal/domain/run"
)

// RunListItemResponse is the minimal shape of GET /workflows/:id/runs members.
type RunListItemResponse struct {
	ID         string           `json:"id"`
	Trigger    string           `json:"trigger"`
	Status     string           `json:"status"`
	Reason     string           `json:"reason,omitempty"`
	StartedAt  string           `json:"startedAt"`
	FinishedAt *string          `json:"finishedAt,omitempty"`
	DurationMs int              `json:"durationMs"`
	Counts     RunCountsSummary `json:"counts"`
}

type RunCountsSummary struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Error   int `json:"error"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

func NewRunListResponseFromViews(views []domainrun.View) []RunListItemResponse {
	out := make([]RunListItemResponse, 0, len(views))
	for _, v := range views {
		out = append(out, newRunListItem(v))
	}
	return out
}

func newRunListItem(v domainrun.View) RunListItemResponse {
	item := RunListItemResponse{
		ID:         v.ID.String(),
		Trigger:    v.Trigger,
		Status:     string(v.Status),
		Reason:     v.Reason,
		StartedAt:  v.StartedAt.UTC().Format(time.RFC3339),
		DurationMs: v.DurationMS,
		Counts: RunCountsSummary{
			Success: v.Counts.Success,
			Failed:  v.Counts.Failed,
			Error:   v.Counts.Error,
			Skipped: v.Counts.Skipped,
			Total:   v.Counts.Total(),
		},
	}
	if !v.FinishedAt.IsZero() {
		s := v.FinishedAt.UTC().Format(time.RFC3339)
		item.FinishedAt = &s
	}
	return item
}

// RunDetailResponse is the full shape of GET /workflows/:id/runs/:runId.
type RunDetailResponse struct {
	ID         string            `json:"id"`
	WorkflowID string            `json:"workflowId"`
	Trigger    string            `json:"trigger"`
	Status     string            `json:"status"`
	Reason     string            `json:"reason,omitempty"`
	StartedAt  string            `json:"startedAt"`
	FinishedAt *string           `json:"finishedAt,omitempty"`
	DurationMs int               `json:"durationMs"`
	Counts     RunCountsSummary  `json:"counts"`
	Variables  map[string]string `json:"variables"`
	Calls      []CallRunResponse `json:"calls"`
}

type CallRunResponse struct {
	ID                 string             `json:"id"`
	CallID             *string            `json:"callId,omitempty"`
	Name               string             `json:"name"`
	Position           int                `json:"position"`
	Status             string             `json:"status"`
	Reason             string             `json:"reason,omitempty"`
	HTTPStatus         int                `json:"httpStatus,omitempty"`
	Request            any                `json:"request"`
	Response           any                `json:"response"`
	Timings            any                `json:"timings"`
	Network            any                `json:"network"`
	TLS                any                `json:"tls,omitempty"`
	Rules              []RuleRunResponse  `json:"rules"`
	ExtractedVariables map[string]string  `json:"extractedVariables,omitempty"`
	UsedVariables      []string           `json:"usedVariables,omitempty"`
	Error              *CallErrorResponse `json:"error,omitempty"`
	BodyRef            string             `json:"bodyRef,omitempty"`
	BodyHash           string             `json:"bodyHash,omitempty"`
	BodyEncoding       string             `json:"bodyEncoding,omitempty"`
	ResponseKeys       []string           `json:"responseKeys,omitempty"`
	StartedAt          *string            `json:"startedAt,omitempty"`
	FinishedAt         *string            `json:"finishedAt,omitempty"`
}

type RuleRunResponse struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Text     string `json:"text"`
	Observed string `json:"observed"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
}

type CallErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func NewRunDetailResponseFromView(v domainrun.DetailView) RunDetailResponse {
	out := RunDetailResponse{
		ID:         v.ID.String(),
		WorkflowID: v.WorkflowID.String(),
		Trigger:    v.Trigger,
		Status:     string(v.Status),
		Reason:     v.Reason,
		StartedAt:  v.StartedAt.UTC().Format(time.RFC3339),
		DurationMs: v.DurationMS,
		Counts: RunCountsSummary{
			Success: v.Counts.Success,
			Failed:  v.Counts.Failed,
			Error:   v.Counts.Error,
			Skipped: v.Counts.Skipped,
			Total:   v.Counts.Total(),
		},
		Variables: v.Variables,
		Calls:     make([]CallRunResponse, 0, len(v.Calls)),
	}
	if out.Variables == nil {
		out.Variables = map[string]string{}
	}
	if !v.FinishedAt.IsZero() {
		s := v.FinishedAt.UTC().Format(time.RFC3339)
		out.FinishedAt = &s
	}
	for _, c := range v.Calls {
		out.Calls = append(out.Calls, newCallRunResponse(c))
	}
	return out
}

func newCallRunResponse(c domainrun.CallRunView) CallRunResponse {
	out := CallRunResponse{
		ID:                 c.ID.String(),
		Name:               c.Name,
		Position:           c.Position,
		Status:             string(c.Status),
		Reason:             c.Reason,
		HTTPStatus:         c.HTTPStatus,
		Request:            c.Request,
		Response:           c.Response,
		Timings:            c.Timings,
		Network:            c.Network,
		TLS:                c.TLS,
		Rules:              make([]RuleRunResponse, 0, len(c.Rules)),
		ExtractedVariables: c.ExtractedVariables,
		UsedVariables:      c.UsedVariables,
		BodyRef:            c.BodyRef,
		BodyHash:           c.BodyHash,
		BodyEncoding:       c.BodyEncoding,
		ResponseKeys:       c.ResponseKeys,
	}
	if c.CallID != nil {
		s := c.CallID.String()
		out.CallID = &s
	}
	if c.Error != nil {
		out.Error = &CallErrorResponse{Type: c.Error.Type, Message: c.Error.Message}
	}
	if c.StartedAt != nil {
		s := c.StartedAt.UTC().Format(time.RFC3339)
		out.StartedAt = &s
	}
	if c.FinishedAt != nil {
		s := c.FinishedAt.UTC().Format(time.RFC3339)
		out.FinishedAt = &s
	}
	for _, rr := range c.Rules {
		out.Rules = append(out.Rules, RuleRunResponse{
			ID: rr.ID.String(), Position: rr.Position, Subject: rr.Subject, Operator: rr.Operator,
			Text: rr.Text, Observed: rr.Observed, Expected: rr.Expected, Passed: rr.Passed,
		})
	}
	return out
}
