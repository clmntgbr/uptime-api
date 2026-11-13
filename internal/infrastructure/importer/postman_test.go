package importer

import (
	"errors"
	"strings"
	"testing"

	"go-api/internal/domain/library"
)

const postmanCollection = `{
  "info": {"name": "Demo", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{token}}"}]},
  "item": [
    {"name": "Users", "item": [
      {"name": "List", "request": {
        "method": "get",
        "url": {"raw": "{{base}}/users?limit=10&x=1", "query": [{"key": "limit", "value": "10"}, {"key": "x", "value": "1", "disabled": true}]},
        "header": [{"key": "Accept", "value": "application/json"}, {"key": "X-Old", "value": "1", "disabled": true}]
      }},
      {"name": "Login", "request": {
        "method": "POST",
        "url": {"raw": "{{base}}/login"},
        "body": {"mode": "urlencoded", "urlencoded": []}
      }, "auth": {"type": "noauth"}}
    ]},
    {"name": "Raw", "request": {
      "method": "POST",
      "url": "https://x.test/raw",
      "body": {"mode": "raw", "raw": "{\"a\":1}"},
      "description": {"content": "Body desc"}
    }},
    {"name": "Key", "request": {
      "method": "GET",
      "url": {"host": ["api", "x", "test"], "path": ["v1", "items"]},
      "auth": {"type": "apikey", "apikey": [{"key": "key", "value": "X-Key"}, {"key": "in", "value": "header"}]}
    }},
    {"name": "Inline", "request": {"method": "GET", "url": {"raw": "https://x.test/p?a=1&b=2"}}}
  ]
}`

func TestParsePostmanCollection(t *testing.T) {
	res, err := NewParser().Parse(library.FormatPostman, []byte(postmanCollection))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 5 || len(res.Skipped) != 0 {
		t.Fatalf("got %d candidates, %d skipped; want 5, 0", len(res.Candidates), len(res.Skipped))
	}
	checkCandidate(t, res.Candidates[0], wantCandidate{
		name:        "List",
		method:      "GET",
		url:         "{{base}}/users",
		query:       map[string]string{"limit": "10"},
		headers:     map[string]string{"Accept": "application/json", "Authorization": "Bearer {{token}}"},
		description: "Folder: Users",
		warnings:    []string{"auth_placeholder"},
	})
	checkCandidate(t, res.Candidates[1], wantCandidate{
		name:        "Login",
		method:      "POST",
		url:         "{{base}}/login",
		description: "Folder: Users",
		warnings:    []string{"unsupported_body"},
	})
	checkCandidate(t, res.Candidates[2], wantCandidate{
		name:        "Raw",
		method:      "POST",
		url:         "https://x.test/raw",
		body:        `{"a":1}`,
		headers:     map[string]string{"Content-Type": "application/json", "Authorization": "Bearer {{token}}"},
		description: "Body desc",
		warnings:    []string{"auth_placeholder"},
	})
	checkCandidate(t, res.Candidates[3], wantCandidate{
		name:     "Key",
		method:   "GET",
		url:      "api.x.test/v1/items",
		headers:  map[string]string{"X-Key": "{{api_key}}"},
		warnings: []string{"auth_placeholder"},
	})
	checkCandidate(t, res.Candidates[4], wantCandidate{
		name:     "Inline",
		method:   "GET",
		url:      "https://x.test/p",
		query:    map[string]string{"a": "1", "b": "2"},
		headers:  map[string]string{"Authorization": "Bearer {{token}}"},
		warnings: []string{"auth_placeholder"},
	})
	if res.Candidates[4].Key != "req-5" {
		t.Errorf("key = %q, want req-5", res.Candidates[4].Key)
	}
}

func TestParsePostmanTruncation(t *testing.T) {
	collection := `{"info": {"schema": "v2.1"}, "item": [{"name": "` + strings.Repeat("n", 150) +
		`", "request": {"method": "GET", "url": "https://x.test/", "description": "` + strings.Repeat("d", 600) + `"}}]}`
	res, err := NewParser().Parse(library.FormatPostman, []byte(collection))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(res.Candidates))
	}
	c := res.Candidates[0]
	if len(c.Definition.Name) != 100 || len(c.Definition.Description) != 500 {
		t.Errorf("name/description lengths = %d/%d, want 100/500", len(c.Definition.Name), len(c.Definition.Description))
	}
	checkCandidate(t, c, wantCandidate{
		name:        strings.Repeat("n", 100),
		method:      "GET",
		url:         "https://x.test/",
		description: strings.Repeat("d", 500),
		warnings:    []string{"name_truncated", "description_truncated"},
	})
}

func TestParsePostmanRejections(t *testing.T) {
	tests := []struct {
		name    string
		content string
		message string
	}{
		{"v2.0", `{"info": {"schema": "https://schema.getpostman.com/json/collection/v2.0.0/collection.json"}, "item": []}`, "Postman v2.0 is not supported"},
		{"not a collection", `{"foo": 1}`, "content is not a Postman collection"},
		{"invalid json", `{"item": [`, "content is not a Postman collection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewParser().Parse(library.FormatPostman, []byte(tt.content))
			var importErr *library.ImportError
			if !errors.As(err, &importErr) {
				t.Fatalf("err = %v, want ImportError", err)
			}
			if importErr.Message != tt.message {
				t.Errorf("message = %q, want %q", importErr.Message, tt.message)
			}
		})
	}
}

func TestParsePostmanSkipsRequestWithoutURL(t *testing.T) {
	collection := `{"info": {"schema": "v2.1"}, "item": [{"name": "Empty", "request": {"method": "GET"}}]}`
	res, err := NewParser().Parse(library.FormatPostman, []byte(collection))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Reason != "no URL" {
		t.Errorf("candidates = %d, skipped = %+v; want 0 and one 'no URL'", len(res.Candidates), res.Skipped)
	}
}
