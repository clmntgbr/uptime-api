package engine

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"go-api/internal/domain/variable"
)

// Headers whose value is always masked, whatever it contains.
var sensitiveHeaders = map[string]struct{}{
	"Authorization":       {},
	"Proxy-Authorization": {},
	"Cookie":              {},
	"Set-Cookie":          {},
	"X-Api-Key":           {},
	"X-Auth-Token":        {},
	"X-Csrf-Token":        {},
}

var secretLikeName = regexp.MustCompile(`(?i)(pass(word|wd)?|secret|token|api[_-]?key|auth|credential|private[_-]?key|session|cookie|signature)`)

// minMaskLength avoids masking every occurrence of a trivially short value ("1", "ok").
const minMaskLength = 4

// masker replaces secret values with ••••. It grows during a run: extracted
// variables with a secret-like name become secrets for the following calls.
type masker struct {
	values []string
}

func newMasker(secrets map[string]string) *masker {
	m := &masker{}
	for _, v := range secrets {
		m.add(v)
	}
	return m
}

func (m *masker) add(value string) {
	if len(value) < minMaskLength {
		return
	}
	for _, existing := range m.values {
		if existing == value {
			return
		}
	}
	m.values = append(m.values, value)
	// Longest first, so a secret containing another one is masked whole.
	sort.Slice(m.values, func(i, j int) bool { return len(m.values[i]) > len(m.values[j]) })
}

func (m *masker) mask(s string) string {
	if s == "" {
		return s
	}
	for _, v := range m.values {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, variable.MaskedValue)
		}
	}
	return s
}

func (m *masker) maskHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = m.maskHeader(k, v)
	}
	return out
}

func (m *masker) maskHTTPHeaders(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, values := range h {
		out[k] = m.maskHeader(k, strings.Join(values, ", "))
	}
	return out
}

func (m *masker) maskHeader(name, value string) string {
	if isSensitiveHeader(name) {
		return variable.MaskedValue
	}
	return m.mask(value)
}

func isSecretLikeName(name string) bool {
	return secretLikeName.MatchString(name)
}

func isSensitiveHeader(name string) bool {
	if _, ok := sensitiveHeaders[http.CanonicalHeaderKey(name)]; ok {
		return true
	}
	return secretLikeName.MatchString(name)
}
