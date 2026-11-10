package call

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNew_RejectsHeaderCRLF(t *testing.T) {
	_, err := New(Definition{
		Name:   "login",
		Method: "GET",
		URL:    "https://example.com",
		Headers: map[string]string{
			"X-Custom": "ok\r\nX-Injected: evil",
		},
	}, 1, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "CR or LF") {
		t.Fatalf("header CRLF must be rejected, got %v", err)
	}
}

func TestNew_TrimsOptionalDescription(t *testing.T) {
	got, err := New(Definition{
		Name:        "login",
		Description: "  checks auth  ",
		Method:      "GET",
		URL:         "https://example.com",
	}, 1, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "checks auth" {
		t.Fatalf("description: got %q", got.Description)
	}
}

func TestNew_RejectsLongDescription(t *testing.T) {
	_, err := New(Definition{
		Name:        "login",
		Description: strings.Repeat("x", MaxDescriptionLength+1),
		Method:      "GET",
		URL:         "https://example.com",
	}, 1, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("long description must be rejected, got %v", err)
	}
}
