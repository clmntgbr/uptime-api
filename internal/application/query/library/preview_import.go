package library

import (
	"context"
	"fmt"

	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type PreviewImportQuery struct {
	UserID  uuid.UUID
	Format  domainlibrary.Format
	Content []byte
	// BaseURL, when set, is applied to every candidate whose URL has no host.
	// May be absolute http(s) or a global variable template whose value is a URL.
	BaseURL string
}

// PreviewImportResult lists what the import would create. Nothing is written.
type PreviewImportResult struct {
	Candidates []domainlibrary.Candidate
	Skipped    []domainlibrary.Skipped
}

type PreviewImportHandler struct {
	importer domainlibrary.Importer
	globals  variable.GlobalRepository
}

func NewPreviewImportHandler(
	importer domainlibrary.Importer,
	globals variable.GlobalRepository,
) *PreviewImportHandler {
	return &PreviewImportHandler{importer: importer, globals: globals}
}

// Handle parses the content and keeps only candidates that pass the same
// validation as a library item. Invalid candidates move to Skipped with the reason.
func (h *PreviewImportHandler) Handle(ctx context.Context, q PreviewImportQuery) (*PreviewImportResult, error) {
	if len(q.Content) > domainlibrary.MaxImportBytes {
		return nil, domainlibrary.ErrImportTooLarge
	}

	lookup, err := h.globalLookup(ctx, q.UserID, q.BaseURL)
	if err != nil {
		return nil, err
	}
	baseURL, err := domainlibrary.ValidateBaseURL(q.BaseURL, lookup)
	if err != nil {
		return nil, err
	}
	parsed, err := h.importer.Parse(q.Format, q.Content)
	if err != nil {
		return nil, err
	}

	if baseURL == "" {
		for _, c := range parsed.Candidates {
			if domainlibrary.NeedsBaseURL(c.Definition.URL) {
				return nil, domainlibrary.ErrBaseURLRequired
			}
		}
	}

	result := &PreviewImportResult{Skipped: parsed.Skipped}
	for _, c := range parsed.Candidates {
		c = domainlibrary.ApplyBaseURL(c, baseURL)
		normalized, err := domainlibrary.Normalize(c.Definition)
		if err != nil {
			result.Skipped = append(result.Skipped, domainlibrary.Skipped{
				Source: fmt.Sprintf("%s (%s)", c.Key, c.Definition.Name),
				Reason: err.Error(),
			})
			continue
		}
		c.Definition = normalized
		result.Candidates = append(result.Candidates, c)
	}
	return result, nil
}

// globalLookup loads the user's globals when baseURL contains placeholders.
func (h *PreviewImportHandler) globalLookup(
	ctx context.Context,
	userID uuid.UUID,
	baseURL string,
) (func(string) (string, bool), error) {
	if !variable.HasReferences(baseURL) {
		return nil, nil
	}
	globals, err := h.globals.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}
	byName := make(map[string]string, len(globals))
	for _, g := range globals {
		byName[g.Name] = g.Value
	}
	return func(name string) (string, bool) {
		v, ok := byName[name]
		return v, ok
	}, nil
}
