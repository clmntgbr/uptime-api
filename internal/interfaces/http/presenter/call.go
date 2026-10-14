package presenter

import (
	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"
)

type CallResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewCallResponse(id, name string) CallResponse {
	return CallResponse{ID: id, Name: name}
}

func NewCallResponseFromCall(c domaincall.Call) CallResponse {
	return NewCallResponse(c.ID.String(), c.Name)
}

func NewCallListResponseFromViews(views []domainworkflow.CallListView) []CallResponse {
	out := make([]CallResponse, 0, len(views))
	for _, v := range views {
		out = append(out, NewCallResponse(v.ID.String(), v.Name))
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
