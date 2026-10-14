package presenter

import (
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
	ID              string `json:"id"`
	Name            string `json:"name"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Enabled         bool   `json:"enabled"`
}

func NewWorkflowResponseFromView(v domainworkflow.View) WorkflowResponse {
	return WorkflowResponse{
		ID:              v.ID.String(),
		Name:            v.Name,
		IntervalSeconds: v.IntervalSeconds,
		Enabled:         v.Enabled,
	}
}
