package library

import (
	"errors"
	"testing"

	domaincall "go-api/internal/domain/call"
)

func TestValidateBaseURL(t *testing.T) {
	lookup := func(name string) (string, bool) {
		switch name {
		case "api_base":
			return "https://api.example.com", true
		case "host":
			return "api.example.com", true
		case "bad":
			return "not-a-url", true
		case "empty":
			return "", true
		default:
			return "", false
		}
	}

	ok := map[string]string{
		"":                             "",
		"  https://api.example.com/  ": "https://api.example.com",
		"http://localhost:8080/v1":     "http://localhost:8080/v1",
		"{{api_base}}":                 "{{api_base}}",
		" {{api_base}}/ ":              "{{api_base}}",
		"{{api_base}}/v1/":             "{{api_base}}/v1",
		"https://{{host}}/api":         "https://{{host}}/api",
	}
	for in, want := range ok {
		got, err := ValidateBaseURL(in, lookup)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}

	for _, bad := range []string{"api.example.com", "ftp://x.com", "https://", "not a url", "prefix{{api_base}}"} {
		if _, err := ValidateBaseURL(bad, lookup); !errors.Is(err, ErrInvalidBaseURL) {
			t.Errorf("%q: expected ErrInvalidBaseURL, got %v", bad, err)
		}
	}

	if _, err := ValidateBaseURL("{{missing}}", lookup); !errors.Is(err, ErrBaseURLUnknownVariable) {
		t.Errorf("missing: expected ErrBaseURLUnknownVariable, got %v", err)
	}
	if _, err := ValidateBaseURL("{{api_base}}", nil); !errors.Is(err, ErrBaseURLUnknownVariable) {
		t.Errorf("nil lookup: expected ErrBaseURLUnknownVariable, got %v", err)
	}
	if _, err := ValidateBaseURL("{{bad}}", lookup); !errors.Is(err, ErrBaseURLVariableNotURL) {
		t.Errorf("bad value: expected ErrBaseURLVariableNotURL, got %v", err)
	}
	if _, err := ValidateBaseURL("{{empty}}", lookup); !errors.Is(err, ErrBaseURLVariableNotURL) {
		t.Errorf("empty value: expected ErrBaseURLVariableNotURL, got %v", err)
	}
}

func TestNeedsBaseURL(t *testing.T) {
	need := []string{"/files", "{{base_url}}/files", "{{base_url}}/pets/{{petId}}"}
	for _, in := range need {
		if !NeedsBaseURL(in) {
			t.Errorf("%q: want NeedsBaseURL true", in)
		}
	}
	keep := []string{"https://api.example.com/x", "https://api.example.com/pets/{{petId}}", "{{tenant}}/x"}
	for _, in := range keep {
		if NeedsBaseURL(in) {
			t.Errorf("%q: want NeedsBaseURL false", in)
		}
	}
}

func TestApplyBaseURL(t *testing.T) {
	base := "https://api.example.com/v1"
	cases := map[string]string{
		"{{base_url}}/files": "https://api.example.com/v1/files",
		"/files":             "https://api.example.com/v1/files",
		"https://other.io/x": "https://other.io/x",
		"{{tenant}}/x":       "{{tenant}}/x",
	}
	for in, want := range cases {
		got := ApplyBaseURL(Candidate{Definition: domaincall.Definition{URL: in}}, base)
		if got.Definition.URL != want {
			t.Errorf("%q: got %q want %q", in, got.Definition.URL, want)
		}
	}

	got := ApplyBaseURL(
		Candidate{Definition: domaincall.Definition{URL: "{{base_url}}/files"}},
		"{{api_base}}",
	)
	if got.Definition.URL != "{{api_base}}/files" {
		t.Errorf("variable base: got %q want {{api_base}}/files", got.Definition.URL)
	}
	got = ApplyBaseURL(
		Candidate{Definition: domaincall.Definition{URL: "/files"}},
		"{{api_base}}",
	)
	if got.Definition.URL != "{{api_base}}/files" {
		t.Errorf("variable base on path: got %q want {{api_base}}/files", got.Definition.URL)
	}
}

func TestApplyBaseURL_EmptyBaseKeepsCandidate(t *testing.T) {
	c := Candidate{Definition: domaincall.Definition{URL: "{{base_url}}/x"}}
	if got := ApplyBaseURL(c, ""); got.Definition.URL != "{{base_url}}/x" {
		t.Fatalf("empty base must not change url, got %q", got.Definition.URL)
	}
}

func TestApplyBaseURL_DropsBaseURLWarningOnly(t *testing.T) {
	c := Candidate{
		Definition: domaincall.Definition{URL: "{{base_url}}/pets/{{petId}}"},
		Warnings: []Warning{
			{Code: "variable_to_define", Field: "url", Message: "Set base_url to the server URL https://x"},
			{Code: "variable_to_define", Field: "url", Message: "Define petId"},
			{Code: "deprecated_operation", Message: "deprecated"},
		},
	}
	got := ApplyBaseURL(c, "https://api.example.com")
	if len(got.Warnings) != 2 || got.Warnings[0].Message != "Define petId" {
		t.Fatalf("unexpected warnings %+v", got.Warnings)
	}
}
