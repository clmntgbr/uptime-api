package importer

import (
	"errors"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"
)

func TestParseCurl(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    wantCandidate
		skipped int
	}{
		{
			name:  "single GET",
			input: `curl https://api.example.com/users`,
			want:  wantCandidate{name: "GET /users", method: "GET", url: "https://api.example.com/users"},
		},
		{
			name: "multi-line with continuations",
			input: "curl -X POST https://api.example.com/items \\\n" +
				"  -H 'Content-Type: application/json' \\\n" +
				"  -d '{\"a\":1}'",
			want: wantCandidate{
				name: "POST /items", method: "POST", url: "https://api.example.com/items",
				headers: map[string]string{"Content-Type": "application/json"},
				body:    `{"a":1}`,
			},
		},
		{
			name:  "ANSI-C quoted body",
			input: `curl 'https://x.test/a' --data-raw $'{"x":"a\nb\x41"}'`,
			want: wantCandidate{
				name: "POST /a", method: "POST", url: "https://x.test/a",
				body: "{\"x\":\"a\nbA\"}",
			},
		},
		{
			name:  "double quotes with escapes",
			input: `curl "https://x.test/q" -H "X-Quoted: say \"hi\""`,
			want: wantCandidate{
				name: "GET /q", method: "GET", url: "https://x.test/q",
				headers: map[string]string{"X-Quoted": `say "hi"`},
			},
		},
		{
			name:  "-G turns data into query",
			input: `curl -G https://x.test/search -d q=hello -d page=2`,
			want: wantCandidate{
				name: "GET /search", method: "GET", url: "https://x.test/search",
				query: map[string]string{"q": "hello", "page": "2"},
			},
		},
		{
			name:  "basic auth",
			input: `curl -u alice:s3cret https://x.test/`,
			want: wantCandidate{
				name: "GET /", method: "GET", url: "https://x.test/",
				headers:  map[string]string{"Authorization": "Basic YWxpY2U6czNjcmV0"},
				warnings: []string{"sensitive_header"},
			},
		},
		{
			name:  "insecure, max-time, proxy, ipv4",
			input: `curl -k -m 2.5 -x http://proxy.test:8080 -4 https://x.test/`,
			want: wantCandidate{
				name: "GET /", method: "GET", url: "https://x.test/",
				verifyTLS: boolPtr(false), timeout: 2500 * time.Millisecond,
				proxy: "http://proxy.test:8080", ipVersion: domaincall.IPv4,
			},
		},
		{
			name:  "grouped short flags and attached values",
			input: `curl -sSLk -XPUT https://x.test/`,
			want: wantCandidate{
				name: "PUT /", method: "PUT", url: "https://x.test/",
				verifyTLS: boolPtr(false), warnings: []string{"unsupported_option"},
			},
		},
		{
			name:  "unknown flag warns",
			input: `curl --frobnicate https://x.test/a`,
			want: wantCandidate{
				name: "GET /a", method: "GET", url: "https://x.test/a",
				warnings: []string{"unsupported_option"},
			},
		},
		{
			name:  "form and file data are unsupported",
			input: `curl -F file=@a.txt https://x.test/up`,
			want: wantCandidate{
				name: "POST /up", method: "POST", url: "https://x.test/up",
				warnings: []string{"unsupported_option"},
			},
		},
		{
			name:  "data file reference gives no body",
			input: `curl https://x.test/up -d @body.json`,
			want: wantCandidate{
				name: "POST /up", method: "POST", url: "https://x.test/up",
				warnings: []string{"unsupported_option"},
			},
		},
		{
			name:  "head and location",
			input: `curl -I -L https://x.test/h`,
			want: wantCandidate{
				name: "HEAD /h", method: "HEAD", url: "https://x.test/h",
				warnings: []string{"unsupported_option"},
			},
		},
		{
			name:  "empty curl-style header is skipped silently",
			input: `curl https://x.test/ -H 'X-Empty;' -s --compressed`,
			want:  wantCandidate{name: "GET /", method: "GET", url: "https://x.test/"},
		},
		{
			name:  "templated URL is accepted",
			input: `curl {{base_url}}/orders`,
			want:  wantCandidate{name: "GET {{base_url}}/orders", method: "GET", url: "{{base_url}}/orders"},
		},
		{
			name:    "no URL is skipped",
			input:   `curl -H 'A: b'`,
			want:    wantCandidate{},
			skipped: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := NewParser().Parse(library.FormatCurl, []byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Skipped) != tt.skipped {
				t.Fatalf("skipped = %+v, want %d", res.Skipped, tt.skipped)
			}
			if tt.skipped > 0 {
				if res.Skipped[0].Reason != "no URL" || res.Skipped[0].Source != "command 1" {
					t.Errorf("skipped = %+v, want no URL for command 1", res.Skipped[0])
				}
				return
			}
			if len(res.Candidates) != 1 {
				t.Fatalf("got %d candidates, want 1", len(res.Candidates))
			}
			checkCandidate(t, res.Candidates[0], tt.want)
		})
	}
}

func TestParseCurlMultipleCommands(t *testing.T) {
	input := "curl https://x.test/a\n\n" +
		"wget https://x.test/skip\n" +
		"curl -X DELETE https://x.test/b\n"
	res, err := NewParser().Parse(library.FormatCurl, []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 || res.Candidates[0].Key != "cmd-1" || res.Candidates[1].Key != "cmd-2" {
		t.Fatalf("candidates = %+v, want cmd-1 and cmd-2", res.Candidates)
	}
	checkCandidate(t, res.Candidates[1], wantCandidate{name: "DELETE /b", method: "DELETE", url: "https://x.test/b"})
	if len(res.Skipped) != 1 || res.Skipped[0].Source != "wget https://x.test/skip" {
		t.Errorf("skipped = %+v", res.Skipped)
	}
}

func TestParseCurlNoCommand(t *testing.T) {
	_, err := NewParser().Parse(library.FormatCurl, []byte(`\`))
	var importErr *library.ImportError
	if !errors.As(err, &importErr) || importErr.Message != "no curl command found" {
		t.Fatalf("err = %v, want 'no curl command found'", err)
	}
}
