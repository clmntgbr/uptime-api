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
