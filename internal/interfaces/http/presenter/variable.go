package presenter

import (
	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
)

type VariableResponse struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
	Path     string `json:"path,omitempty"`
	Source   string `json:"source,omitempty"`
	CallID   string `json:"callId,omitempty"`
	CallName string `json:"callName,omitempty"`
}

func NewGlobalVariableResponse(g variable.Global) VariableResponse {
	value := g.Value
	if g.Secret {
		value = variable.MaskedValue
	}
	return VariableResponse{
		ID:     g.ID.String(),
		Name:   g.Name,
		Type:   string(variable.TypeGlobal),
		Value:  value,
		Secret: g.Secret,
	}
}

func NewGlobalVariableListResponse(globals []variable.Global) []VariableResponse {
	out := make([]VariableResponse, 0, len(globals))
	for _, g := range globals {
		out = append(out, NewGlobalVariableResponse(g))
	}
	return out
}

func NewStaticVariableResponse(s domainworkflow.StaticVariable) VariableResponse {
	value := s.Value
	if s.Secret {
		value = variable.MaskedValue
	}
	return VariableResponse{
		Name:   s.Name,
		Type:   string(variable.TypeStatic),
		Value:  value,
		Secret: s.Secret,
	}
}

func NewStaticVariableListResponse(statics []domainworkflow.StaticVariable) []VariableResponse {
	out := make([]VariableResponse, 0, len(statics))
	for _, s := range statics {
		out = append(out, NewStaticVariableResponse(s))
	}
	return out
}

func NewCallVariableResponse(e domaincall.Extraction) VariableResponse {
	return VariableResponse{
		Name:   e.Name,
		Type:   string(variable.TypeCall),
		Path:   e.Expression,
		Source: string(e.Source),
		Secret: e.Secret,
	}
}

func NewCallVariableListResponse(extractions []domaincall.Extraction) []VariableResponse {
	out := make([]VariableResponse, 0, len(extractions))
	for _, e := range extractions {
		out = append(out, NewCallVariableResponse(e))
	}
	return out
}

func NewAvailableVariableResponse(a variable.Available) VariableResponse {
	value := a.Value
	if a.Secret {
		value = variable.MaskedValue
	}
	resp := VariableResponse{
		Name:     a.Name,
		Type:     string(a.Type),
		Value:    value,
		Secret:   a.Secret,
		Path:     a.Path,
		Source:   a.Source,
		CallName: a.CallName,
	}
	if a.Type == variable.TypeCall {
		resp.CallID = a.CallID.String()
	}
	return resp
}

func NewAvailableVariableListResponse(items []variable.Available) []VariableResponse {
	out := make([]VariableResponse, 0, len(items))
	for _, a := range items {
		out = append(out, NewAvailableVariableResponse(a))
	}
	return out
}
