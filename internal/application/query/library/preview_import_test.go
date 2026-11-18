package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type fakeImporter struct {
	result domainlibrary.ImportResult
	err    error
}

func (f fakeImporter) Parse(_ domainlibrary.Format, _ []byte) (domainlibrary.ImportResult, error) {
	return f.result, f.err
}

type fakeGlobals struct {
	items []variable.Global
	err   error
}

func (f fakeGlobals) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (f fakeGlobals) ListByUser(context.Context, uuid.UUID) ([]variable.Global, error) {
	return f.items, f.err
}
func (f fakeGlobals) ListForContext(context.Context, uuid.UUID) ([]variable.Global, error) {
	return nil, nil
}
func (f fakeGlobals) FindByID(context.Context, uuid.UUID, uuid.UUID) (*variable.Global, error) {
	return nil, nil
}
func (f fakeGlobals) FindByName(context.Context, uuid.UUID, string) (*variable.Global, error) {
	return nil, nil
}
func (f fakeGlobals) Names(context.Context, uuid.UUID) ([]string, error) { return nil, nil }
func (f fakeGlobals) Save(context.Context, *variable.Global) error       { return nil }
func (f fakeGlobals) Delete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

func TestPreviewImport_AppliesBaseURLAndValidates(t *testing.T) {
	imp := fakeImporter{result: domainlibrary.ImportResult{Candidates: []domainlibrary.Candidate{
		{Key: "op-1", Definition: domaincall.Definition{Name: "a", URL: "{{base_url}}/files", Method: "GET"}},
		{Key: "op-2", Definition: domaincall.Definition{Name: "b", URL: "https://other.io/x", Method: "GET"}},
	}}}
	h := NewPreviewImportHandler(imp, fakeGlobals{})

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
	h := NewPreviewImportHandler(fakeImporter{}, fakeGlobals{})
	_, err := h.Handle(context.Background(), PreviewImportQuery{Content: []byte("x"), BaseURL: "api.example.com"})
	if !errors.Is(err, domainlibrary.ErrInvalidBaseURL) {
		t.Fatalf("expected ErrInvalidBaseURL, got %v", err)
	}
}

func TestPreviewImport_RequiresBaseURLWhenCandidatesHaveNoHost(t *testing.T) {
	imp := fakeImporter{result: domainlibrary.ImportResult{Candidates: []domainlibrary.Candidate{
		{Key: "op-1", Definition: domaincall.Definition{Name: "a", URL: "{{base_url}}/files", Method: "GET"}},
		{Key: "op-2", Definition: domaincall.Definition{Name: "b", URL: "https://other.io/x", Method: "GET"}},
	}}}
	h := NewPreviewImportHandler(imp, fakeGlobals{
		items: []variable.Global{{Name: "api_base", Value: "https://api.example.com"}},
	})
	_, err := h.Handle(context.Background(), PreviewImportQuery{Content: []byte("x")})
	if !errors.Is(err, domainlibrary.ErrBaseURLRequired) {
		t.Fatalf("expected ErrBaseURLRequired, got %v", err)
	}

	res, err := h.Handle(context.Background(), PreviewImportQuery{
		Content: []byte("x"), BaseURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 || res.Candidates[0].Definition.URL != "https://api.example.com/files" {
		t.Fatalf("base url should unlock candidates: %+v", res.Candidates)
	}

	res, err = h.Handle(context.Background(), PreviewImportQuery{
		UserID: uuid.New(), Content: []byte("x"), BaseURL: "{{api_base}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Candidates[0].Definition.URL; got != "{{api_base}}/files" {
		t.Fatalf("variable base not applied: %q", got)
	}
}

func TestPreviewImport_RejectsVariableBaseThatIsNotURL(t *testing.T) {
	h := NewPreviewImportHandler(fakeImporter{}, fakeGlobals{
		items: []variable.Global{{Name: "api_base", Value: "not-a-url"}},
	})
	_, err := h.Handle(context.Background(), PreviewImportQuery{
		UserID: uuid.New(), Content: []byte("x"), BaseURL: "{{api_base}}",
	})
	if !errors.Is(err, domainlibrary.ErrBaseURLVariableNotURL) {
		t.Fatalf("expected ErrBaseURLVariableNotURL, got %v", err)
	}
}

func TestPreviewImport_RejectsUnknownVariableBase(t *testing.T) {
	h := NewPreviewImportHandler(fakeImporter{}, fakeGlobals{})
	_, err := h.Handle(context.Background(), PreviewImportQuery{
		UserID: uuid.New(), Content: []byte("x"), BaseURL: "{{missing}}",
	})
	if !errors.Is(err, domainlibrary.ErrBaseURLUnknownVariable) {
		t.Fatalf("expected ErrBaseURLUnknownVariable, got %v", err)
	}
}

func TestPreviewImport_InvalidCandidateGoesToSkipped(t *testing.T) {
	imp := fakeImporter{result: domainlibrary.ImportResult{Candidates: []domainlibrary.Candidate{
		{Key: "op-1", Definition: domaincall.Definition{Name: "bad", URL: "", Method: "GET"}},
	}}}
	res, err := NewPreviewImportHandler(imp, fakeGlobals{}).Handle(context.Background(), PreviewImportQuery{Content: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Source, "op-1") {
		t.Fatalf("invalid candidate must be skipped: %+v", res)
	}
}

func TestPreviewImport_TooLargeAndParserError(t *testing.T) {
	h := NewPreviewImportHandler(fakeImporter{}, fakeGlobals{})
	big := make([]byte, domainlibrary.MaxImportBytes+1)
	if _, err := h.Handle(context.Background(), PreviewImportQuery{Content: big}); !errors.Is(err, domainlibrary.ErrImportTooLarge) {
		t.Fatalf("expected ErrImportTooLarge, got %v", err)
	}
	h = NewPreviewImportHandler(fakeImporter{err: &domainlibrary.ImportError{Message: "boom"}}, fakeGlobals{})
	if _, err := h.Handle(context.Background(), PreviewImportQuery{Content: []byte("x")}); err == nil {
		t.Fatal("parser error must propagate")
	}
}
