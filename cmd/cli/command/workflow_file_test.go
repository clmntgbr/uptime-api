package command

import (
	"errors"
	"strings"
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

const ordersFile = `
workflow: orders-api
interval: 5m
static:
  base_url: https://api.example.com
  tenant: acme
calls:
  - name: login
    method: POST
    url: "{{base_url}}/auth/token"
    body: '{"secret": "{{api_secret}}"}'
    extract:
      token: $.access_token
      session: header:Set-Cookie
    rules:
      - status is 200
      - response time exceeds 800 ms
  - name: list
    url: "{{base_url}}/orders"
    timeout: 10s
    headers:
      Authorization: "Bearer {{token}}"
    extract:
      first_id: $.items[0].id
      code: status
      ref: 'regex:ref=([a-z0-9]+)'
    rules:
      - status is 200
      - json $.items exists
  - name: detail
    url: "{{base_url}}/orders/{{first_id}}"
    rules:
      - certificate expires in less than 14 days
`

func TestParseWorkflowFile(t *testing.T) {
	def, err := parseWorkflowFile([]byte(ordersFile))
	if err != nil {
		t.Fatal(err)
	}
	if def.Name != "orders-api" || def.Interval != 5*time.Minute || !def.Enabled || len(def.Calls) != 3 {
		t.Fatalf("unexpected definition %+v", def)
	}
	if def.Statics[0].Name != "base_url" || def.Statics[1].Name != "tenant" {
		t.Fatalf("static order must follow the file: %+v", def.Statics)
	}
	login := def.Calls[0]
	if login.Extractions[0].Source != domaincall.ExtractJSON || login.Extractions[1].Source != domaincall.ExtractHeader ||
		login.Extractions[1].Expression != "Set-Cookie" {
		t.Fatalf("unexpected extractions %+v", login.Extractions)
	}
	list := def.Calls[1]
	if list.Timeout != 10*time.Second || list.Extractions[1].Source != domaincall.ExtractStatus ||
		list.Extractions[2].Source != domaincall.ExtractRegex || list.Extractions[2].Expression != "ref=([a-z0-9]+)" {
		t.Fatalf("unexpected call %+v", list)
	}

	if _, err := domainworkflow.New(uuid.New(), def, []string{"api_secret"}); err != nil {
		t.Fatalf("definition must be valid: %v", err)
	}
}

func TestParseWorkflowFile_OrderIsChecked(t *testing.T) {
	def, err := parseWorkflowFile([]byte(ordersFile))
	if err != nil {
		t.Fatal(err)
	}
	def.Calls[0], def.Calls[1] = def.Calls[1], def.Calls[0]
	_, err = domainworkflow.New(uuid.New(), def, []string{"api_secret"})
	if !errors.Is(err, domainworkflow.ErrVarOutOfOrder) || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected out of order error naming token, got %v", err)
	}
}

func TestParseWorkflowFile_Errors(t *testing.T) {
	cases := map[string]string{
		"unknown field": "workflow: x\ninterval: 5m\nschedule: daily\ncalls: []\n",
		"bad interval":  "workflow: x\ninterval: often\ncalls: []\n",
		"bad rule":      "workflow: x\ninterval: 5m\ncalls:\n  - name: a\n    url: https://x.io\n    rules: [latency is low]\n",
		"bad timeout":   "workflow: x\ninterval: 5m\ncalls:\n  - name: a\n    url: https://x.io\n    timeout: soon\n",
		"nested static": "workflow: x\ninterval: 5m\nstatic:\n  a: {b: c}\ncalls: []\n",
	}
	for name, content := range cases {
		if _, err := parseWorkflowFile([]byte(content)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
