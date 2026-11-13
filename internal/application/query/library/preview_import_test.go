package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
)

type fakeImporter struct {
	result domainlibrary.ImportResult
	err    error
}

func (f fakeImporter) Parse(_ domainlibrary.Format, _ []byte) (domainlibrary.ImportResult, error) {
	return f.result, f.err
}

func TestPreviewImport_AppliesBaseURLAndValidates(t *testing.T) {
	imp := fakeImporter{result: domainlibrary.ImportResult{Candidates: []domainlibrary.Candidate{
		{Key: "op-1", Definition: domaincall.Definition{Name: "a", URL: "{{base_url}}/files", Method: "GET"}},
		{Key: "op-2", Definition: domaincall.Definition{Name: "b", URL: "https://other.io/x", Method: "GET"}},
	}}}
	h := NewPreviewImportHandler(imp)

	res, err := h.Handle(context.Background(), PreviewImportQuery{
		Format: domainlibrary.FormatOpenAPI, Content: []byte("x"), BaseURL: " https://api.example.com/v1/ ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(res.Candidates))
	}
	if got := res.Candidates[0].Definition.URL; got != "https://api.example.com/v1/files" {
		t.Fatalf("base url not applied: %q", got)
	}
	if got := res.Candidates[1].Definition.URL; got != "https://other.io/x" {
		t.Fatalf("absolute url must be kept: %q", got)
	}
}

func TestPreviewImport_RejectsInvalidBaseURL(t *testing.T) {
	h := NewPreviewImportHandler(fakeImporter{})
	_, err := h.Handle(context.Background(), PreviewImportQuery{Content: []byte("x"), BaseURL: "api.example.com"})
	if !errors.Is(err, domainlibrary.ErrInvalidBaseURL) {
		t.Fatalf("expected ErrInvalidBaseURL, got %v", err)
	}
}

func TestPreviewImport_InvalidCandidateGoesToSkipped(t *testing.T) {
	imp := fakeImporter{result: domainlibrary.ImportResult{Candidates: []domainlibrary.Candidate{
		{Key: "op-1", Definition: domaincall.Definition{Name: "bad", URL: "", Method: "GET"}},
	}}}
	res, err := NewPreviewImportHandler(imp).Handle(context.Background(), PreviewImportQuery{Content: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Source, "op-1") {
		t.Fatalf("invalid candidate must be skipped: %+v", res)
	}
}

func TestPreviewImport_TooLargeAndParserError(t *testing.T) {
	h := NewPreviewImportHandler(fakeImporter{})
	big := make([]byte, domainlibrary.MaxImportBytes+1)
	if _, err := h.Handle(context.Background(), PreviewImportQuery{Content: big}); !errors.Is(err, domainlibrary.ErrImportTooLarge) {
		t.Fatalf("expected ErrImportTooLarge, got %v", err)
	}
	h = NewPreviewImportHandler(fakeImporter{err: &domainlibrary.ImportError{Message: "boom"}})
	if _, err := h.Handle(context.Background(), PreviewImportQuery{Content: []byte("x")}); err == nil {
		t.Fatal("parser error must propagate")
	}
}
