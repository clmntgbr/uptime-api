package presenter

import (
	"time"

	domainrun "go-api/internal/domain/run"
	domainworkflow "go-api/internal/domain/workflow"
)

// ContextLatestRunResponse is GET /workflows/:id/context/runs/latest.
type ContextLatestRunResponse struct {
	Run *ContextRunResponse `json:"run"`
}

type ContextRunResponse struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	StartedAt  string                 `json:"startedAt"`
	DurationMs int                    `json:"durationMs"`
	Calls      []ContextCallResponse  `json:"calls"`
}

type ContextCallResponse struct {
	CallID     *string `json:"callId,omitempty"`
	Name       string  `json:"name"`
	Position   int     `json:"position"`
	Status     string  `json:"status"`
	OffsetMs   int64   `json:"offsetMs"`
	DurationMs int64   `json:"durationMs"`
	HTTPStatus *int    `json:"httpStatus,omitempty"`
}

func NewContextLatestRunResponse(view *domainrun.LatestView) ContextLatestRunResponse {
	if view == nil {
		return ContextLatestRunResponse{Run: nil}
	}
	calls := make([]ContextCallResponse, 0, len(view.Calls))
	for _, c := range view.Calls {
		item := ContextCallResponse{
			Name:       c.Name,
			Position:   c.Position,
			Status:     string(c.Status),
			OffsetMs:   c.OffsetMS,
			DurationMs: c.DurationMS,
			HTTPStatus: c.HTTPStatus,
		}
		if c.CallID != nil {
			s := c.CallID.String()
			item.CallID = &s
		}
		calls = append(calls, item)
	}
	return ContextLatestRunResponse{
		Run: &ContextRunResponse{
			ID:         view.ID.String(),
			Status:     string(view.Status),
			StartedAt:  view.StartedAt.UTC().Format(time.RFC3339),
			DurationMs: view.DurationMS,
			Calls:      calls,
		},
	}
}

// ContextVariablesResponse is GET /workflows/:id/context/variables.
type ContextVariablesResponse struct {
	Variables []ContextVariableResponse `json:"variables"`
}

type ContextVariableResponse struct {
	Scope      string                     `json:"scope"`
	Name       string                     `json:"name"`
	Value      *string                    `json:"value"`
	Secret     bool                       `json:"secret"`
	Truncated  bool                       `json:"truncated,omitempty"`
	ProducedBy *ContextProducerResponse   `json:"producedBy,omitempty"`
	Status     string                     `json:"status,omitempty"`
}

type ContextProducerResponse struct {
	CallID string `json:"callId"`
	Name   string `json:"name"`
}

func NewContextVariablesResponse(views []domainworkflow.ContextVariableView) ContextVariablesResponse {
	out := make([]ContextVariableResponse, 0, len(views))
	for _, v := range views {
		item := ContextVariableResponse{
			Scope:     v.Scope,
			Name:      v.Name,
			Value:     v.Value,
			Secret:    v.Secret,
			Truncated: v.Truncated,
			Status:    v.Status,
		}
		if v.ProducedBy != nil {
			item.ProducedBy = &ContextProducerResponse{
				CallID: v.ProducedBy.CallID.String(),
				Name:   v.ProducedBy.Name,
			}
		}
		out = append(out, item)
	}
	return ContextVariablesResponse{Variables: out}
}
