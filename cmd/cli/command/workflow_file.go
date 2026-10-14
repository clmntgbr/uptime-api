package command

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	domaincall "go-api/internal/domain/call"
	domainrule "go-api/internal/domain/rule"
	domainworkflow "go-api/internal/domain/workflow"

	"gopkg.in/yaml.v3"
)

// workflowFile is the YAML definition format:
//
//	workflow: orders-api
//	interval: 5m
//	static:
//	  base_url: https://api.example.com
//	calls:
//	  - name: login
//	    method: POST
//	    url: "{{base_url}}/auth/token"
//	    body: '{"secret": "{{api_secret}}"}'
//	    extract:
//	      token: $.access_token
//	    rules:
//	      - status is 200
//
// Extractions are `$.path` (JSONPath on the body), `header:Name`, `status` or `regex:<expression>`.
type workflowFile struct {
	Workflow string             `yaml:"workflow"`
	Interval string             `yaml:"interval"`
	Enabled  *bool              `yaml:"enabled"`
	Static   yaml.Node          `yaml:"static"`
	Calls    []workflowFileCall `yaml:"calls"`
}

type workflowFileCall struct {
	Name      string            `yaml:"name"`
	Method    string            `yaml:"method"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
	Query     map[string]string `yaml:"query"`
	Body      string            `yaml:"body"`
	Timeout   string            `yaml:"timeout"`
	VerifyTLS *bool             `yaml:"verify_tls"`
	IPVersion string            `yaml:"ip_version"`
	Proxy     string            `yaml:"proxy"`
	Extract   yaml.Node         `yaml:"extract"`
	Rules     []string          `yaml:"rules"`
}

func parseWorkflowFile(data []byte) (domainworkflow.Definition, error) {
	var file workflowFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return domainworkflow.Definition{}, fmt.Errorf("invalid workflow file: %w", err)
	}

	interval, err := time.ParseDuration(strings.TrimSpace(file.Interval))
	if err != nil {
		return domainworkflow.Definition{}, fmt.Errorf("interval: %q is not a duration (1m, 5m, 1h)", file.Interval)
	}

	statics, err := orderedPairs(&file.Static, "static")
	if err != nil {
		return domainworkflow.Definition{}, err
	}
	def := domainworkflow.Definition{
		Name:         file.Workflow,
		ScheduleType: domainworkflow.ScheduleInterval,
		Interval:     interval,
		Enabled:      file.Enabled == nil || *file.Enabled,
	}
	for _, p := range statics {
		def.Statics = append(def.Statics, domainworkflow.StaticVariable{Name: p.key, Value: p.value})
	}

	for i, c := range file.Calls {
		call, err := parseCall(c)
		if err != nil {
			name := c.Name
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			return domainworkflow.Definition{}, fmt.Errorf("call %s: %w", name, err)
		}
		def.Calls = append(def.Calls, call)
	}
	return def, nil
}

func parseCall(c workflowFileCall) (domaincall.Definition, error) {
	def := domaincall.Definition{
		Name:      c.Name,
		Method:    c.Method,
		URL:       c.URL,
		Headers:   c.Headers,
		Query:     c.Query,
		Body:      c.Body,
		VerifyTLS: c.VerifyTLS,
		IPVersion: domaincall.IPVersion(strings.ToLower(c.IPVersion)),
		ProxyURL:  c.Proxy,
	}
	if c.Timeout != "" {
		timeout, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return def, fmt.Errorf("timeout: %q is not a duration (500ms, 10s)", c.Timeout)
		}
		def.Timeout = timeout
	}

	extractions, err := orderedPairs(&c.Extract, "extract")
	if err != nil {
		return def, err
	}
	for _, p := range extractions {
		def.Extractions = append(def.Extractions, parseExtraction(p.key, p.value))
	}

	for _, text := range c.Rules {
		rule, err := domainrule.Parse(text)
		if err != nil {
			return def, err
		}
		def.Rules = append(def.Rules, rule)
	}
	return def, nil
}

func parseExtraction(name, expression string) domaincall.Extraction {
	expression = strings.TrimSpace(expression)
	lower := strings.ToLower(expression)
	switch {
	case lower == "status":
		return domaincall.Extraction{Name: name, Source: domaincall.ExtractStatus}
	case strings.HasPrefix(lower, "header:"):
		return domaincall.Extraction{Name: name, Source: domaincall.ExtractHeader, Expression: strings.TrimSpace(expression[len("header:"):])}
	case strings.HasPrefix(lower, "regex:"):
		return domaincall.Extraction{Name: name, Source: domaincall.ExtractRegex, Expression: expression[len("regex:"):]}
	default:
		return domaincall.Extraction{Name: name, Source: domaincall.ExtractJSON, Expression: expression}
	}
}

type pair struct{ key, value string }

// orderedPairs reads a YAML mapping keeping the file order, which a Go map would lose.
func orderedPairs(node *yaml.Node, field string) ([]pair, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping of name: value", field)
	}
	out := make([]pair, 0, len(node.Content)/2)
	seen := map[string]struct{}{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i], node.Content[i+1]
		if v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s.%s must be a string", field, k.Value)
		}
		if _, dup := seen[k.Value]; dup {
			return nil, fmt.Errorf("%s.%s is declared twice", field, k.Value)
		}
		seen[k.Value] = struct{}{}
		out = append(out, pair{key: k.Value, value: v.Value})
	}
	return out, nil
}
