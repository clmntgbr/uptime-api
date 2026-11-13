package library

import (
	"context"
	"fmt"

	domainlibrary "go-api/internal/domain/library"
)

type PreviewImportQuery struct {
	Format  domainlibrary.Format
	Content []byte
	// BaseURL, when set, is applied to every candidate whose URL has no host.
	BaseURL string
}

// PreviewImportResult lists what the import would create. Nothing is written.
type PreviewImportResult struct {
	Candidates []domainlibrary.Candidate
	Skipped    []domainlibrary.Skipped
}

type PreviewImportHandler struct {
	importer domainlibrary.Importer
}

func NewPreviewImportHandler(importer domainlibrary.Importer) *PreviewImportHandler {
	return &PreviewImportHandler{importer: importer}
}

// Handle parses the content and keeps only candidates that pass the same
// validation as a library item. Invalid candidates move to Skipped with the reason.
func (h *PreviewImportHandler) Handle(_ context.Context, q PreviewImportQuery) (*PreviewImportResult, error) {
	if len(q.Content) > domainlibrary.MaxImportBytes {
		return nil, domainlibrary.ErrImportTooLarge
	}
	baseURL, err := domainlibrary.ValidateBaseURL(q.BaseURL)
	if err != nil {
		return nil, err
	}
	parsed, err := h.importer.Parse(q.Format, q.Content)
	if err != nil {
		return nil, err
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
