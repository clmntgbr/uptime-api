package call

import (
	"fmt"
	"regexp"
	"strings"

	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type ExtractionSource string

const (
	// ExtractJSON reads a JSONPath from the body: `$.access_token`.
	ExtractJSON ExtractionSource = "json"
	// ExtractHeader reads a response header: `Set-Cookie`.
	ExtractHeader ExtractionSource = "header"
	// ExtractStatus reads the status code.
	ExtractStatus ExtractionSource = "status"
	// ExtractRegex applies a regular expression to the body; the first capture
	// group is used when present, the whole match otherwise.
	ExtractRegex ExtractionSource = "regex"
)

// Extraction declares a `call` variable produced from the call response.
type Extraction struct {
	ID         uuid.UUID
	Name       string
	Source     ExtractionSource
	Expression string
	// Secret marks the extracted value as secret (masked in results, sealed when reused).
	Secret bool
}

// NormalizeExtraction validates and normalizes an extraction declaration.
// When Source is empty, status is assumed if Expression is empty, otherwise json.
func NormalizeExtraction(e Extraction) (Extraction, error) {
	e.Name = strings.TrimSpace(e.Name)
	e.Expression = strings.TrimSpace(e.Expression)
	if e.Source == "" {
		if e.Expression == "" {
			e.Source = ExtractStatus
		} else {
			e.Source = ExtractJSON
		}
	}
	return normalizeExtraction(e)
}

func normalizeExtraction(e Extraction) (Extraction, error) {
	e.Name = strings.TrimSpace(e.Name)
	e.Expression = strings.TrimSpace(e.Expression)
	if err := variable.ValidateName(e.Name); err != nil {
		return Extraction{}, err
	}
	switch e.Source {
	case ExtractJSON:
		if !strings.HasPrefix(e.Expression, "$") {
			return Extraction{}, fmt.Errorf("extraction %q: JSONPath must start with $", e.Name)
		}
	case ExtractHeader:
		if e.Expression == "" || strings.ContainsAny(e.Expression, " \t:") {
			return Extraction{}, fmt.Errorf("extraction %q: header name is required", e.Name)
		}
	case ExtractStatus:
		e.Expression = ""
	case ExtractRegex:
		if _, err := regexp.Compile(e.Expression); err != nil || e.Expression == "" {
			return Extraction{}, fmt.Errorf("extraction %q: invalid regular expression", e.Name)
		}
	default:
		return Extraction{}, fmt.Errorf("extraction %q: unknown source %q", e.Name, e.Source)
	}
	return e, nil
}
