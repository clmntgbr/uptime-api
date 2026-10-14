package presenter

import (
	"time"

	domainworkflow "go-api/internal/domain/workflow"
)

type WorkflowIDResponse struct {
	ID string `json:"id"`
}

func NewWorkflowIDResponse(w domainworkflow.Workflow) WorkflowIDResponse {
	return WorkflowIDResponse{ID: w.ID.String()}
}

// WorkflowListItemResponse is the shape of GET /workflows members.
type WorkflowListItemResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewWorkflowListResponseFromViews(views []domainworkflow.ListView) []WorkflowListItemResponse {
	out := make([]WorkflowListItemResponse, 0, len(views))
	for _, v := range views {
		out = append(out, WorkflowListItemResponse{ID: v.ID.String(), Name: v.Name})
	}
	return out
}

// WorkflowResponse is the shape of GET /workflows/:id.
type WorkflowResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ScheduleType    string  `json:"scheduleType"`
	IntervalSeconds *int    `json:"intervalSeconds,omitempty"`
	RunAt           *string `json:"runAt,omitempty"`
	Enabled         bool    `json:"enabled"`
}

func NewWorkflowResponseFromView(v domainworkflow.View) WorkflowResponse {
	out := WorkflowResponse{
		ID:           v.ID.String(),
		Name:         v.Name,
		ScheduleType: string(v.ScheduleType),
		Enabled:      v.Enabled,
	}
	if out.ScheduleType == "" {
		out.ScheduleType = string(domainworkflow.ScheduleInterval)
	}
	if v.ScheduleType == domainworkflow.ScheduleInterval || v.ScheduleType == "" {
		seconds := v.IntervalSeconds
		out.IntervalSeconds = &seconds
	}
	if v.RunAt != nil {
		s := v.RunAt.UTC().Format(time.RFC3339)
		out.RunAt = &s
	}
	return out
}

// WorkflowRunResponse is the shape of POST /workflows/:id/runs.
type WorkflowRunResponse struct {
	RunID string `json:"runId"`
}

func NewWorkflowRunResponse(runID string) WorkflowRunResponse {
	return WorkflowRunResponse{RunID: runID}
}
