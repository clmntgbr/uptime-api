package presenter

import (
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
)

// LibraryRuleResponse is one alert rule of a library definition.
type LibraryRuleResponse struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Target   string `json:"target,omitempty"`
	Value    string `json:"value,omitempty"`
}

// LibraryDefinitionResponse is the request shape of a library item (same fields as CallRequest).
type LibraryDefinitionResponse struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Method      string                `json:"method"`
	URL         string                `json:"url"`
	Headers     map[string]string     `json:"headers"`
	Query       map[string]string     `json:"query"`
	Body        string                `json:"body"`
	TimeoutMS   int64                 `json:"timeoutMs"`
	VerifyTLS   bool                  `json:"verifyTls"`
	IPVersion   string                `json:"ipVersion"`
	ProxyURL    string                `json:"proxyUrl"`
	Rules       []LibraryRuleResponse `json:"rules"`
}

// LibraryItemResponse is the shape of GET/POST/PUT /library items and import results.
type LibraryItemResponse struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	LibraryDefinitionResponse
}

// LibraryListItemResponse is one row of GET /library.
type LibraryListItemResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// LibraryWarningResponse is one remark on an import candidate.
type LibraryWarningResponse struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// LibraryImportCandidateResponse is one candidate of an import preview.
type LibraryImportCandidateResponse struct {
	Key      string                    `json:"key"`
	Payload  LibraryDefinitionResponse `json:"payload"`
	Warnings []LibraryWarningResponse  `json:"warnings"`
}

// LibrarySkippedResponse is a source element that produced no candidate.
type LibrarySkippedResponse struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// LibraryImportPreviewResponse is the body of POST /library/import/preview.
type LibraryImportPreviewResponse struct {
	Items   []LibraryImportCandidateResponse `json:"items"`
	Skipped []LibrarySkippedResponse         `json:"skipped"`
}

func NewLibraryDefinitionResponse(def domaincall.Definition) LibraryDefinitionResponse {
	verifyTLS := def.VerifyTLS == nil || *def.VerifyTLS
	rules := make([]LibraryRuleResponse, 0, len(def.Rules))
	for _, r := range def.Rules {
		rules = append(rules, LibraryRuleResponse{
			Subject: string(r.Subject), Operator: string(r.Operator), Target: r.Target, Value: r.Value,
		})
	}
	return LibraryDefinitionResponse{
		Name:        def.Name,
		Description: def.Description,
		Method:      def.Method,
		URL:         def.URL,
		Headers:     nonNilMap(def.Headers),
		Query:       nonNilMap(def.Query),
		Body:        def.Body,
		TimeoutMS:   def.Timeout.Milliseconds(),
		VerifyTLS:   verifyTLS,
		IPVersion:   string(def.IPVersion),
		ProxyURL:    def.ProxyURL,
		Rules:       rules,
	}
}

func NewLibraryItemResponse(item domainlibrary.Item) LibraryItemResponse {
	return LibraryItemResponse{
		ID:                        item.ID.String(),
		CreatedAt:                 item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 item.UpdatedAt.UTC().Format(time.RFC3339),
		LibraryDefinitionResponse: NewLibraryDefinitionResponse(item.Definition),
	}
}

func NewLibraryItemListResponse(items []domainlibrary.Item) []LibraryItemResponse {
	out := make([]LibraryItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, NewLibraryItemResponse(item))
	}
	return out
}

func NewLibraryListItemResponses(views []domainlibrary.ListView) []LibraryListItemResponse {
	out := make([]LibraryListItemResponse, 0, len(views))
	for _, v := range views {
		out = append(out, LibraryListItemResponse{
			ID:          v.ID.String(),
			Name:        v.Name,
			Description: v.Description,
			Method:      v.Method,
			URL:         v.URL,
			CreatedAt:   v.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:   v.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func NewLibraryImportPreviewResponse(candidates []domainlibrary.Candidate, skipped []domainlibrary.Skipped) LibraryImportPreviewResponse {
	items := make([]LibraryImportCandidateResponse, 0, len(candidates))
	for _, c := range candidates {
		warnings := make([]LibraryWarningResponse, 0, len(c.Warnings))
		for _, w := range c.Warnings {
			warnings = append(warnings, LibraryWarningResponse{Code: w.Code, Field: w.Field, Message: w.Message})
		}
		items = append(items, LibraryImportCandidateResponse{
			Key:      c.Key,
			Payload:  NewLibraryDefinitionResponse(c.Definition),
			Warnings: warnings,
		})
	}
	skips := make([]LibrarySkippedResponse, 0, len(skipped))
	for _, s := range skipped {
		skips = append(skips, LibrarySkippedResponse{Source: s.Source, Reason: s.Reason})
	}
	return LibraryImportPreviewResponse{Items: items, Skipped: skips}
}

func nonNilMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	return in
}
