// Package importer converts OpenAPI documents, cURL commands and Postman
// collections into call library candidates. It performs no I/O.
package importer

import (
	"bytes"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"
)

// Warning codes. They are part of the preview contract: keep them stable.
const (
	warnAuthPlaceholder   = "auth_placeholder"
	warnCookieParameter   = "cookie_parameter_ignored"
	warnDeprecated        = "deprecated_operation"
	warnDescriptionTrunc  = "description_truncated"
	warnNameTrunc         = "name_truncated"
	warnSensitiveHeader   = "sensitive_header"
	warnUnsupportedBody   = "unsupported_body"
	warnUnsupportedOption = "unsupported_option"
	warnVariableRenamed   = "variable_renamed"
	warnVariableToDefine  = "variable_to_define"
)

const (
	maxVariableLength = 64
	variableChars     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
)

var validVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

var sensitiveHeaderNames = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"cookie":              true,
	"set-cookie":          true,
}

// Parser implements library.Importer for OpenAPI, cURL and Postman sources.
type Parser struct{}

var _ library.Importer = (*Parser)(nil)

// NewParser returns a Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse converts content in the given format into candidates. Content that
// cannot be read as that format yields a *library.ImportError.
func (p *Parser) Parse(format library.Format, content []byte) (library.ImportResult, error) {
	switch format {
	case library.FormatOpenAPI, library.FormatCurl, library.FormatPostman:
	default:
		return library.ImportResult{}, library.ErrUnsupportedFormat
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return library.ImportResult{}, importError("import content is empty")
	}
	switch format {
	case library.FormatOpenAPI:
		return parseOpenAPI(content)
	case library.FormatCurl:
		return parseCurl(content)
	default:
		return parsePostman(content)
	}
}

// importError returns an ImportError with a deliberate, client-safe message.
func importError(message string) error {
	return &library.ImportError{Message: message}
}

// warningList accumulates the warnings of one candidate.
type warningList []library.Warning

func (w *warningList) add(code, field, format string, args ...any) {
	*w = append(*w, library.Warning{Code: code, Field: field, Message: fmt.Sprintf(format, args...)})
}

// addCandidate appends a candidate keyed "<prefix>-<n>", where n is its
// 1-based position among candidates. Names and descriptions are truncated and
// sensitive literal headers are flagged here, so every format shares the rules.
func addCandidate(res *library.ImportResult, prefix string, def domaincall.Definition, ws warningList) {
	name, truncated := truncateRunes(strings.TrimSpace(def.Name), domaincall.MaxNameLength)
	if truncated {
		ws.add(warnNameTrunc, "name", "Name is longer than %d characters and was truncated", domaincall.MaxNameLength)
	}
	description, truncated := truncateRunes(strings.TrimSpace(def.Description), domaincall.MaxDescriptionLength)
	if truncated {
		ws.add(warnDescriptionTrunc, "description", "Description is longer than %d characters and was truncated", domaincall.MaxDescriptionLength)
	}
	def.Name, def.Description = name, description
	if len(def.Headers) == 0 {
		def.Headers = nil
	}
	if len(def.Query) == 0 {
		def.Query = nil
	}
	addSensitiveHeaderWarnings(def.Headers, &ws)
	res.Candidates = append(res.Candidates, library.Candidate{
		Key:        fmt.Sprintf("%s-%d", prefix, len(res.Candidates)+1),
		Definition: def,
		Warnings:   ws,
	})
}

// addSkipped records a source element that produced no candidate.
func addSkipped(res *library.ImportResult, source, reason string) {
	res.Skipped = append(res.Skipped, library.Skipped{Source: source, Reason: reason})
}

// addSensitiveHeaderWarnings flags sensitive headers that carry a literal value.
// Values are imported as written; placeholders are not literal and are not flagged.
func addSensitiveHeaderWarnings(headers map[string]string, ws *warningList) {
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		value := headers[name]
		if isSensitiveHeader(name) && value != "" && !strings.Contains(value, "{{") {
			ws.add(warnSensitiveHeader, "headers."+name, "Header %s has a literal value; it is imported as written", name)
		}
	}
}

// setAuthPlaceholder adds an authentication placeholder as a header or query
// parameter, unless the request already sets that key. No secret is inserted.
func setAuthPlaceholder(def *domaincall.Definition, in, name, value string, ws *warningList) {
	switch in {
	case "header":
		if hasHeader(def.Headers, name) {
			return
		}
		def.Headers[name] = value
		ws.add(warnAuthPlaceholder, "headers."+name, "Authentication is set to the placeholder %s; provide the value in the workflow", value)
	case "query":
		if _, exists := def.Query[name]; exists {
			return
		}
		def.Query[name] = value
		ws.add(warnAuthPlaceholder, "query."+name, "Authentication is set to the placeholder %s; provide the value in the workflow", value)
	}
}

// templateVariable returns name as a valid variable name, warning when it changed.
func templateVariable(name, field string, ws *warningList) string {
	variable, changed := variableName(name)
	if changed {
		ws.add(warnVariableRenamed, field, "Name %q is not a valid variable name and was renamed to %q", name, variable)
	}
	return variable
}

// variableName returns name as a valid workflow variable name and reports
// whether it had to change: invalid characters become "_", a leading digit or
// "-" gets a "_" prefix, and the result is truncated to maxVariableLength.
func variableName(name string) (string, bool) {
	if validVariableName.MatchString(name) && len(name) <= maxVariableLength {
		return name, false
	}
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(variableChars, r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') || s[0] == '-' {
		s = "_" + s
	}
	if len(s) > maxVariableLength {
		s = s[:maxVariableLength]
	}
	return s, true
}

// truncateRunes shortens s to at most limit runes and reports whether it did.
func truncateRunes(s string, limit int) (string, bool) {
	if utf8.RuneCountInString(s) <= limit {
		return s, false
	}
	return string([]rune(s)[:limit]), true
}

// isSensitiveHeader reports whether a header usually carries a credential.
func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	if sensitiveHeaderNames[lower] {
		return true
	}
	for _, marker := range []string{"token", "secret", "api-key", "apikey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// endpointName returns "METHOD path" for an absolute URL, or "METHOD url" when
// the URL has no host (templated or unparsable).
func endpointName(method, rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return method + " " + rawURL
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return method + " " + path
}

// parseQueryPairs splits "k=v&k2=v2" into a map, keeping keys and values as written.
func parseQueryPairs(raw string) map[string]string {
	query := map[string]string{}
	for _, pair := range strings.Split(raw, "&") {
		key, value, _ := strings.Cut(pair, "=")
		if key == "" {
			continue
		}
		query[key] = value
	}
	return query
}

// setHeader sets a header, replacing any existing header with the same name
// regardless of case.
func setHeader(headers map[string]string, name, value string) {
	for key := range headers {
		if strings.EqualFold(key, name) {
			delete(headers, key)
		}
	}
	headers[name] = value
}

// hasHeader reports whether a header with that name is set, regardless of case.
func hasHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

// stringValue renders a decoded YAML or JSON scalar as text; nil becomes "".
func stringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asList(v any) ([]any, bool) {
	list, ok := v.([]any)
	return list, ok
}
