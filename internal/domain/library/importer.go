package library

import (
	domaincall "go-api/internal/domain/call"
)

// Format is the source format of an import.
type Format string

const (
	FormatOpenAPI Format = "openapi"
	FormatCurl    Format = "curl"
	FormatPostman Format = "postman"
)

// Warning is a remark on one candidate, shown during preview. Warnings never
// block the import: the user decides after reading them.
type Warning struct {
	Code    string // stable machine code, e.g. "sensitive_header"
	Field   string // payload field concerned, e.g. "headers.Authorization"; may be empty
	Message string // English, user-facing
}

// Candidate is one call found in the source, before validation.
type Candidate struct {
	Key        string // stable within one preview, e.g. "op-1"
	Definition domaincall.Definition
	Warnings   []Warning
}

// Skipped is a source element that produced no candidate.
type Skipped struct {
	Source string // e.g. "POST /legacy" or "command 2"
	Reason string
}

// ImportResult is what a parser found in the content.
type ImportResult struct {
	Candidates []Candidate
	Skipped    []Skipped
}

// Importer parses an import source. Implementations do no I/O and perform no
// network access: the content is entirely provided by the caller.
type Importer interface {
	// Parse returns ErrUnsupportedFormat for unknown formats, *ImportError when
	// the content cannot be read as that format. Content larger than
	// MaxImportBytes must be rejected before parsing.
	Parse(format Format, content []byte) (ImportResult, error)
}

// ImportError reports content that cannot be read as the requested format.
// Message is deliberate and safe to show to the client.
type ImportError struct {
	Message string
}

func (e *ImportError) Error() string { return e.Message }
