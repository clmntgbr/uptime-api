package call

import (
	"strings"
	"testing"
)

func TestNormalizeExtraction_JSONPath(t *testing.T) {
	ok, err := NormalizeExtraction(Extraction{Name: "orderId", Source: ExtractJSON, Expression: "$[0].id"})
	if err != nil || ok.Expression != "$[0].id" {
		t.Fatalf("valid path: %+v %v", ok, err)
	}

	_, err = NormalizeExtraction(Extraction{Name: "orderId", Source: ExtractJSON, Expression: "$.[0].id"})
	if err == nil {
		t.Fatal("$.[0].id must be rejected")
	}
}

func TestNormalizeExtraction_RegexPatternLength(t *testing.T) {
	_, err := NormalizeExtraction(Extraction{
		Name: "token", Source: ExtractRegex, Expression: strings.Repeat("a", MaxRegexPatternBytes+1),
	})
	if err == nil {
		t.Fatal("oversized regex pattern must be rejected")
	}
	ok, err := NormalizeExtraction(Extraction{
		Name: "token", Source: ExtractRegex, Expression: `token=(?P<t>[A-Za-z0-9]+)`,
	})
	if err != nil || ok.Source != ExtractRegex {
		t.Fatalf("valid regex: %+v %v", ok, err)
	}
}
