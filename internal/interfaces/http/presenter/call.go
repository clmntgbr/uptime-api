package presenter

import (
	"time"

	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"
)

// CallRecentRunTimingsResponse is the phase breakdown of one sparkline bar.
type CallRecentRunTimingsResponse struct {
	DNS      int64 `json:"dns"`
	Connect  int64 `json:"connect"`
	TLS      int64 `json:"tls"`
	Transfer int64 `json:"transfer"`
}

// CallRecentRunResponse is one bar of the call list sparkline.
type CallRecentRunResponse struct {
	ID         string                         `json:"id"`
	RunID      string                         `json:"runId"`
	Status     string                         `json:"status"`
	HTTPStatus *int                           `json:"httpStatus,omitempty"`
	FinishedAt *string                        `json:"finishedAt,omitempty"`
	LatencyMs  *int64                         `json:"latencyMs,omitempty"`
	Timings    *CallRecentRunTimingsResponse `json:"timings,omitempty"`
}

// CallResponse is the shape of create/update and list call rows.
type CallResponse struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Method         string                   `json:"method"`
	URL            string                   `json:"url"`
	Position       int                      `json:"position"`
	LastStatus     *string                  `json:"lastStatus,omitempty"`
	LastHTTPStatus *int                     `json:"lastHttpStatus,omitempty"`
	LastFinishedAt *string                  `json:"lastFinishedAt,omitempty"`
	LastLatencyMs  *int64                   `json:"lastLatencyMs,omitempty"`
	RecentRuns     []CallRecentRunResponse `json:"recentRuns"`
}

func NewCallResponseFromCall(c domaincall.Call) CallResponse {
	return CallResponse{
		ID:         c.ID.String(),
		Name:       c.Name,
		Method:     c.Method,
		URL:        c.URL,
		Position:   c.Position,
		RecentRuns: []CallRecentRunResponse{},
	}
}

func NewCallListResponseFromCalls(calls []domaincall.Call) []CallResponse {
	out := make([]CallResponse, 0, len(calls))
	for _, c := range calls {
		out = append(out, NewCallResponseFromCall(c))
	}
	return out
}

func NewCallListResponseFromViews(views []domainworkflow.CallListView) []CallResponse {
	out := make([]CallResponse, 0, len(views))
	for _, v := range views {
		out = append(out, NewCallListResponseFromView(v))
	}
	return out
}

func NewCallListResponseFromView(v domainworkflow.CallListView) CallResponse {
	out := CallResponse{
		ID:         v.ID.String(),
		Name:       v.Name,
		Method:     v.Method,
		URL:        v.URL,
		Position:   v.Position,
		RecentRuns: make([]CallRecentRunResponse, 0, len(v.RecentRuns)),
	}
	if v.LastStatus != "" {
		status := v.LastStatus
		out.LastStatus = &status
	}
	out.LastHTTPStatus = v.LastHTTPStatus
	if v.LastFinishedAt != nil {
		s := v.LastFinishedAt.UTC().Format(time.RFC3339)
		out.LastFinishedAt = &s
	}
	out.LastLatencyMs = v.LastLatencyMs
	for _, r := range v.RecentRuns {
		item := CallRecentRunResponse{
			ID:         r.ID.String(),
			RunID:      r.RunID.String(),
			Status:     r.Status,
			HTTPStatus: r.HTTPStatus,
			LatencyMs:  r.LatencyMs,
		}
		if r.FinishedAt != nil {
			s := r.FinishedAt.UTC().Format(time.RFC3339)
			item.FinishedAt = &s
		}
		if r.Timings != nil {
			item.Timings = &CallRecentRunTimingsResponse{
				DNS:      r.Timings.DNS,
				Connect:  r.Timings.Connect,
				TLS:      r.Timings.TLS,
				Transfer: r.Timings.Transfer,
			}
		}
		out.RecentRuns = append(out.RecentRuns, item)
	}
	return out
}

// CallDetailResponse is the shape of GET /workflows/:id/calls/:callId.
type CallDetailResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Query     map[string]string `json:"query"`
	Body      string            `json:"body"`
	TimeoutMS int               `json:"timeoutMs"`
	VerifyTLS bool              `json:"verifyTls"`
	IPVersion string            `json:"ipVersion"`
	ProxyURL  string            `json:"proxyUrl"`
}

func NewCallDetailResponseFromView(v domainworkflow.CallView) CallDetailResponse {
	headers := v.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	query := v.Query
	if query == nil {
		query = map[string]string{}
	}
	return CallDetailResponse{
		ID:        v.ID.String(),
		Name:      v.Name,
		Method:    v.Method,
		URL:       v.URL,
		Headers:   headers,
		Query:     query,
		Body:      v.Body,
		TimeoutMS: v.TimeoutMS,
		VerifyTLS: v.VerifyTLS,
		IPVersion: v.IPVersion,
		ProxyURL:  v.ProxyURL,
	}
}
