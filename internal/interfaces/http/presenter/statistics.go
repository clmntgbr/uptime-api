package presenter

import (
	"time"

	domainstatistics "go-api/internal/domain/statistics"
)

type DashboardStatisticsResponse struct {
	WindowHours   int                          `json:"windowHours"`
	Workflows     WorkflowCountsResponse       `json:"workflows"`
	Runs          RunCountsResponse            `json:"runs"`
	Calls         CallCountsResponse           `json:"calls"`
	SuccessRate   float64                      `json:"successRate"`
	LatencyP95Ms  *int64                       `json:"latencyP95Ms"`
	OpenIncident  bool                         `json:"openIncident"`
	Incidents     []IncidentRangeResponse      `json:"incidents"`
	CallDurations []CallDurationSeriesResponse `json:"callDurations"`
}

type WorkflowCountsResponse struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Disabled int64 `json:"disabled"`
}

type RunCountsResponse struct {
	Total   int64                  `json:"total"`
	Success int64                  `json:"success"`
	Failed  int64                  `json:"failed"`
	Error   int64                  `json:"error"`
	Skipped int64                  `json:"skipped"`
	Running int64                  `json:"running"`
	Daily   []RunDayCountsResponse `json:"daily"`
}

type RunDayCountsResponse struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
	Error   int64  `json:"error"`
	Skipped int64  `json:"skipped"`
	Running int64  `json:"running"`
}

type CallCountsResponse struct {
	Total   int64                   `json:"total"`
	Success int64                   `json:"success"`
	Failed  int64                   `json:"failed"`
	Error   int64                   `json:"error"`
	Skipped int64                   `json:"skipped"`
	Daily   []CallDayCountsResponse `json:"daily"`
}

type CallDayCountsResponse struct {
	Date         string `json:"date"`
	Total        int64  `json:"total"`
	Success      int64  `json:"success"`
	Failed       int64  `json:"failed"`
	Error        int64  `json:"error"`
	Skipped      int64  `json:"skipped"`
	LatencyP95Ms *int64 `json:"latencyP95Ms"`
}

type CallDurationPointResponse struct {
	Date       string `json:"date"`
	DurationMs int64  `json:"durationMs"`
	Samples    int64  `json:"samples"`
}

type CallDurationSeriesResponse struct {
	ID       string                      `json:"id"`
	Name     string                      `json:"name"`
	Position int                         `json:"position"`
	Daily    []CallDurationPointResponse `json:"daily"`
}

type IncidentRangeResponse struct {
	Start string  `json:"start"`
	End   *string `json:"end"`
}

func NewDashboardStatisticsResponse(s domainstatistics.DashboardStatistics) DashboardStatisticsResponse {
	runDaily := make([]RunDayCountsResponse, 0, len(s.Runs.Daily))
	for _, d := range s.Runs.Daily {
		runDaily = append(runDaily, RunDayCountsResponse{
			Date:    d.Date.UTC().Format(time.DateOnly),
			Total:   d.Total,
			Success: d.Success,
			Failed:  d.Failed,
			Error:   d.Error,
			Skipped: d.Skipped,
			Running: d.Running,
		})
	}
	callDaily := make([]CallDayCountsResponse, 0, len(s.Calls.Daily))
	for _, d := range s.Calls.Daily {
		callDaily = append(callDaily, CallDayCountsResponse{
			Date:         d.Date.UTC().Format(time.DateOnly),
			Total:        d.Total,
			Success:      d.Success,
			Failed:       d.Failed,
			Error:        d.Error,
			Skipped:      d.Skipped,
			LatencyP95Ms: d.LatencyP95Ms,
		})
	}
	callDurations := make([]CallDurationSeriesResponse, 0, len(s.CallDurations))
	for _, series := range s.CallDurations {
		daily := make([]CallDurationPointResponse, 0, len(series.Daily))
		for _, point := range series.Daily {
			daily = append(daily, CallDurationPointResponse{
				Date:       point.Date.UTC().Format(time.DateOnly),
				DurationMs: point.DurationMs,
				Samples:    point.Samples,
			})
		}
		callDurations = append(callDurations, CallDurationSeriesResponse{
			ID:       series.ID,
			Name:     series.Name,
			Position: series.Position,
			Daily:    daily,
		})
	}
	incidents := make([]IncidentRangeResponse, 0, len(s.Incidents))
	for _, inc := range s.Incidents {
		item := IncidentRangeResponse{Start: inc.Start.UTC().Format(time.RFC3339)}
		if inc.End != nil {
			end := inc.End.UTC().Format(time.RFC3339)
			item.End = &end
		}
		incidents = append(incidents, item)
	}
	return DashboardStatisticsResponse{
		WindowHours: s.WindowHours,
		Workflows: WorkflowCountsResponse{
			Total: s.Workflows.Total, Active: s.Workflows.Active, Disabled: s.Workflows.Disabled,
		},
		Runs: RunCountsResponse{
			Total: s.Runs.Total, Success: s.Runs.Success, Failed: s.Runs.Failed,
			Error: s.Runs.Error, Skipped: s.Runs.Skipped, Running: s.Runs.Running,
			Daily: runDaily,
		},
		Calls: CallCountsResponse{
			Total: s.Calls.Total, Success: s.Calls.Success, Failed: s.Calls.Failed,
			Error: s.Calls.Error, Skipped: s.Calls.Skipped, Daily: callDaily,
		},
		SuccessRate:   s.SuccessRate,
		LatencyP95Ms:  s.LatencyP95Ms,
		OpenIncident:  s.OpenIncident,
		Incidents:     incidents,
		CallDurations: callDurations,
	}
}
