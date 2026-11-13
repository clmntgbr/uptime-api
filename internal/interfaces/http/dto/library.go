package dto

// InsertFromLibraryRequest is the optional body of POST /workflows/:id/calls/from-library/:itemId.
// Position is 1-based; omitted means the end of the workflow.
type InsertFromLibraryRequest struct {
	Position *int `json:"position" validate:"omitempty,min=1,max=20"`
}

// LibraryImportPreviewRequest is the body of POST /library/import/preview.
// Content is the raw OpenAPI document, Postman collection or cURL command(s).
type LibraryImportPreviewRequest struct {
	Format  string `json:"format" validate:"required,oneof=openapi curl postman"`
	Content string `json:"content" validate:"required"`
	// BaseURL is applied to candidates without their own host, e.g. https://api.example.com.
	BaseURL string `json:"baseUrl" validate:"omitempty,max=2048"`
}

// LibraryImportRequest is the body of POST /library/import: the reviewed candidates to store.
type LibraryImportRequest struct {
	Items []LibraryImportItemRequest `json:"items" validate:"required,min=1,max=50,dive"`
}

// LibraryImportItemRequest is one confirmed candidate. Name, when set, overrides payload.name.
type LibraryImportItemRequest struct {
	Key     string      `json:"key" validate:"max=64"`
	Name    string      `json:"name" validate:"omitempty,max=100"`
	Payload CallRequest `json:"payload"`
}
