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

func NewCallResponseFromView(v domainworkflow.CallListView) CallResponse {
	return NewCallResponse(v.ID.String(), v.Name)
}

func NewCallListResponseFromViews(views []domainworkflow.CallListView) []CallResponse {
	out := make([]CallResponse, 0, len(views))
	for _, v := range views {
		out = append(out, NewCallResponseFromView(v))
	}
	return out
}
