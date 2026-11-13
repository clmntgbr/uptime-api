package library

import (
	"errors"
	"net/url"
	"strings"
)

// ErrInvalidBaseURL is returned when the base URL given at import is not an absolute http(s) URL.
var ErrInvalidBaseURL = errors.New("base url must be an absolute http(s) URL")

// MaxBaseURLLength matches the call URL limit.
const MaxBaseURLLength = 2048

// BaseURLVariable is the placeholder the OpenAPI parser writes for the server URL.
const BaseURLVariable = "{{base_url}}"

// ValidateBaseURL trims raw and checks it is an absolute http(s) URL with a host.
func ValidateBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(trimmed) > MaxBaseURLLength {
		return "", ErrInvalidBaseURL
	}
	return trimmed, nil
}

// ApplyBaseURL makes a candidate URL absolute with baseURL, unless the candidate
// already has its own host:
//   - "{{base_url}}/x" → "<baseURL>/x" (the variable warning is dropped)
//   - "/x" (path only) → "<baseURL>/x"
//   - "https://…" or any other "{{var}}" prefix → unchanged
//
// An empty baseURL leaves the candidate untouched.
func ApplyBaseURL(c Candidate, baseURL string) Candidate {
	if baseURL == "" {
		return c
	}
	def := c.Definition
	switch {
	case strings.HasPrefix(def.URL, BaseURLVariable):
		def.URL = baseURL + strings.TrimPrefix(def.URL, BaseURLVariable)
		c.Warnings = dropBaseURLWarning(c.Warnings)
	case strings.HasPrefix(def.URL, "/"):
		def.URL = baseURL + def.URL
	}
	c.Definition = def
	return c
}

func dropBaseURLWarning(in []Warning) []Warning {
	out := make([]Warning, 0, len(in))
	for _, w := range in {
		if w.Code == "variable_to_define" && strings.Contains(w.Message, "base_url") {
			continue
		}
		out = append(out, w)
	}
	return out
}
