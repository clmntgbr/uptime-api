package library

import (
	"errors"
	"net/url"
	"strings"

	"go-api/internal/domain/variable"
)

// ErrInvalidBaseURL is returned when the base URL given at import is neither an
// absolute http(s) URL nor a well-formed variable template.
var ErrInvalidBaseURL = errors.New("base url must be an absolute http(s) URL or a variable template")

// ErrBaseURLRequired is returned when at least one candidate has no host of its own
// and the import preview was called without a base URL.
var ErrBaseURLRequired = errors.New("base url is required because some calls have no absolute URL")

// ErrBaseURLUnknownVariable is returned when the base URL references a global
// variable that does not exist for the user.
var ErrBaseURLUnknownVariable = errors.New("base url references an unknown global variable")

// ErrBaseURLVariableNotURL is returned when a templated base URL resolves to a
// value that is not an absolute http(s) URL.
var ErrBaseURLVariableNotURL = errors.New("base url variable must resolve to an absolute http(s) URL")

// MaxBaseURLLength matches the call URL limit.
const MaxBaseURLLength = 2048

// BaseURLVariable is the placeholder the OpenAPI parser writes for the server URL.
const BaseURLVariable = "{{base_url}}"

// ValidateBaseURL trims raw and accepts either:
//   - an absolute http(s) URL with a host, or
//   - a template ({{api_base}}, https://{{host}}/v1) whose every placeholder
//     resolves via lookup to a value that makes the whole string an absolute
//     http(s) URL.
//
// The returned string keeps placeholders (lookup is only for validation).
// lookup may be nil when raw has no placeholders.
func ValidateBaseURL(raw string, lookup func(name string) (string, bool)) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > MaxBaseURLLength {
		return "", ErrInvalidBaseURL
	}

	if variable.HasReferences(trimmed) {
		prefix := variable.LiteralPrefix(trimmed)
		if prefix != "" && !strings.HasPrefix(prefix, "http://") && !strings.HasPrefix(prefix, "https://") {
			return "", ErrInvalidBaseURL
		}
		if lookup == nil {
			return "", ErrBaseURLUnknownVariable
		}
		resolved, missing := variable.Render(trimmed, lookup)
		if len(missing) > 0 {
			return "", ErrBaseURLUnknownVariable
		}
		if !isAbsoluteHTTPURL(resolved) {
			return "", ErrBaseURLVariableNotURL
		}
		return trimmed, nil
	}

	if !isAbsoluteHTTPURL(trimmed) {
		return "", ErrInvalidBaseURL
	}
	return trimmed, nil
}

func isAbsoluteHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// NeedsBaseURL reports whether the candidate URL has no host of its own
// (path-only or the {{base_url}} placeholder). Path params like {{id}} do not count.
func NeedsBaseURL(raw string) bool {
	u := strings.TrimSpace(raw)
	return strings.HasPrefix(u, BaseURLVariable) || strings.HasPrefix(u, "/")
}

// ApplyBaseURL prefixes a candidate URL with baseURL, unless the candidate
// already has its own host. baseURL may be absolute or a variable template:
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
