package variable

import (
	"regexp"
	"strings"
)

// placeholder matches `{{name}}`, tolerating inner spaces: `{{ name }}`.
var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// References returns the distinct variable names used in s, in order of appearance.
func References(s string) []string {
	matches := placeholder.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		names = append(names, m[1])
	}
	return names
}

// HasReferences reports whether s contains at least one placeholder.
func HasReferences(s string) bool {
	return placeholder.MatchString(s)
}

// Render replaces every placeholder with lookup(name). Unknown names are left
// untouched and returned in missing (deduplicated).
func Render(s string, lookup func(name string) (string, bool)) (string, []string) {
	var missing []string
	seen := map[string]struct{}{}
	out := placeholder.ReplaceAllStringFunc(s, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if value, ok := lookup(name); ok {
			return value
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			missing = append(missing, name)
		}
		return match
	})
	return out, missing
}

// LiteralPrefix returns the part of s before its first placeholder.
func LiteralPrefix(s string) string {
	loc := placeholder.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]]
}
