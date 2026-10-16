package presenter

import domainstats "go-api/internal/domain/stats"

type DashboardStatsResponse struct {
	WindowHours int                    `json:"windowHours"`
	Workflows   WorkflowCountsResponse `json:"workflows"`
	Runs        RunCountsResponse      `json:"runs"`
	Calls       CallCountsResponse     `json:"calls"`
	SuccessRate float64                `json:"successRate"`
}

type WorkflowCountsResponse struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
}

type RunCountsResponse struct {
	Total   int64 `json:"total"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Error   int64 `json:"error"`
	Skipped int64 `json:"skipped"`
	Running int64 `json:"running"`
}

type CallCountsResponse struct {
	Total   int64 `json:"total"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Error   int64 `json:"error"`
	Skipped int64 `json:"skipped"`
}

func NewDashboardStatsResponse(s domainstats.DashboardStats) DashboardStatsResponse {
	return DashboardStatsResponse{
		WindowHours: s.WindowHours,
		Workflows: WorkflowCountsResponse{
			Total: s.Workflows.Total, Active: s.Workflows.Active, Disabled: s.Workflows.Disabled,
		},
		Runs: RunCountsResponse{
			Total: s.Runs.Total, Success: s.Runs.Success, Failed: s.Runs.Failed,
			Error: s.Runs.Error, Skipped: s.Runs.Skipped, Running: s.Runs.Running,
		},
		Calls: CallCountsResponse{
			Total: s.Calls.Total, Success: s.Calls.Success, Failed: s.Calls.Failed,
			Error: s.Calls.Error, Skipped: s.Calls.Skipped,
		},
		SuccessRate: s.SuccessRate,
	}
}
