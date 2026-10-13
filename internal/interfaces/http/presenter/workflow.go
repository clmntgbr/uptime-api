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

type WorkflowResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	IntervalSeconds int        `json:"intervalSeconds"`
	Enabled         bool       `json:"enabled"`
	Status          *string    `json:"status"`
	NextRunAt       *time.Time `json:"nextRunAt"`
	LastRunAt       *time.Time `json:"lastRunAt"`
	LastStatus      *string    `json:"lastStatus"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func NewWorkflowResponseFromView(v domainworkflow.View) WorkflowResponse {
	return WorkflowResponse{
		ID:              v.ID.String(),
		Name:            v.Name,
		IntervalSeconds: v.IntervalSeconds,
		Enabled:         v.Enabled,
		Status:          optionalNonEmptyString(string(v.Status)),
		NextRunAt:       v.NextRunAt,
		LastRunAt:       v.LastRunAt,
		LastStatus:      optionalNonEmptyString(v.LastStatus),
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
}
