package importer

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"
)

// wantCandidate lists the expected fields; zero values mean "absent".
type wantCandidate struct {
	name, method, url, body, description string
	headers, query                       map[string]string
	timeout                              time.Duration
	verifyTLS                            *bool
	ipVersion                            domaincall.IPVersion
	proxy                                string
	warnings                             []string
}

func checkCandidate(t *testing.T, got library.Candidate, want wantCandidate) {
	t.Helper()
	d := got.Definition
	check := func(field string, g, w any) {
		t.Helper()
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s: %s = %#v, want %#v", got.Key, field, g, w)
		}
	}
	check("name", d.Name, want.name)
	check("method", d.Method, want.method)
	check("url", d.URL, want.url)
	check("body", d.Body, want.body)
	check("description", d.Description, want.description)
	check("headers", d.Headers, want.headers)
	check("query", d.Query, want.query)
	check("timeout", d.Timeout, want.timeout)
	check("verifyTLS", d.VerifyTLS, want.verifyTLS)
	check("ipVersion", d.IPVersion, want.ipVersion)
	check("proxy", d.ProxyURL, want.proxy)
	if len(d.Rules) != 0 {
		t.Errorf("%s: rules = %v, want none", got.Key, d.Rules)
	}

	codes := make([]string, 0, len(got.Warnings))
	for _, w := range got.Warnings {
		codes = append(codes, w.Code)
	}
	wantCodes := slices.Clone(want.warnings)
	slices.Sort(codes)
	slices.Sort(wantCodes)
	if !slices.Equal(codes, wantCodes) {
		t.Errorf("%s: warnings = %v, want %v", got.Key, codes, wantCodes)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestParseRejectsUnsupportedFormat(t *testing.T) {
	_, err := NewParser().Parse(library.Format("har"), []byte("{}"))
	if !errors.Is(err, library.ErrUnsupportedFormat) {
		t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestParseRejectsEmptyContent(t *testing.T) {
	formats := []library.Format{library.FormatOpenAPI, library.FormatCurl, library.FormatPostman}
	for _, content := range []string{"", " \n\t"} {
		for _, format := range formats {
			_, err := NewParser().Parse(format, []byte(content))
			var importErr *library.ImportError
			if !errors.As(err, &importErr) {
				t.Errorf("%s with %q: err = %v, want ImportError", format, content, err)
			}
		}
	}
}

func TestVariableName(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		renamed bool
	}{
		{"user_id", "user_id", false},
		{"ok-name1", "ok-name1", false},
		{"user id", "user_id", true},
		{"1abc", "_1abc", true},
		{"-x", "_-x", true},
		{"", "_", true},
		{"café", "caf_", true},
		{strings.Repeat("a", 70), strings.Repeat("a", 64), true},
	}
	for _, tt := range tests {
		got, renamed := variableName(tt.in)
		if got != tt.want || renamed != tt.renamed {
			t.Errorf("variableName(%q) = %q, %v; want %q, %v", tt.in, got, renamed, tt.want, tt.renamed)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	got, truncated := truncateRunes(strings.Repeat("é", 5), 3)
	if got != "ééé" || !truncated {
		t.Errorf("truncateRunes = %q, %v; want %q, true", got, truncated, "ééé")
	}
	got, truncated = truncateRunes("abc", 3)
	if got != "abc" || truncated {
		t.Errorf("truncateRunes(abc, 3) = %q, %v; want abc, false", got, truncated)
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	tests := map[string]bool{
		"Authorization":   true,
		"x-api-key":       true,
		"X-Client-Secret": true,
		"My-Token":        true,
		"Cookie":          true,
		"Accept":          false,
		"Content-Type":    false,
	}
	for name, want := range tests {
		if got := isSensitiveHeader(name); got != want {
			t.Errorf("isSensitiveHeader(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestSensitiveLiteralHeaderWarning(t *testing.T) {
	res, err := NewParser().Parse(library.FormatCurl, []byte(`curl https://x.test/ -H 'X-Api-Key: abc' -H 'Accept: */*'`))
	if err != nil {
		t.Fatal(err)
	}
	checkCandidate(t, res.Candidates[0], wantCandidate{
		name:     "GET /",
		method:   "GET",
		url:      "https://x.test/",
		headers:  map[string]string{"X-Api-Key": "abc", "Accept": "*/*"},
		warnings: []string{"sensitive_header"},
	})
}
