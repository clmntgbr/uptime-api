package library

import (
	"errors"
	"testing"

	domaincall "go-api/internal/domain/call"
)

func TestValidateBaseURL(t *testing.T) {
	ok := map[string]string{
		"":                             "",
		"  https://api.example.com/  ": "https://api.example.com",
		"http://localhost:8080/v1":     "http://localhost:8080/v1",
	}
	for in, want := range ok {
		got, err := ValidateBaseURL(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"api.example.com", "ftp://x.com", "https://", "not a url"} {
		if _, err := ValidateBaseURL(bad); !errors.Is(err, ErrInvalidBaseURL) {
			t.Errorf("%q: expected ErrInvalidBaseURL, got %v", bad, err)
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
