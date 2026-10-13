package call

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"go-api/internal/domain/rule"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

const (
	DefaultTimeout = 30 * time.Second
	MaxTimeout     = 60 * time.Second
	MinTimeout     = 100 * time.Millisecond
	MaxNameLength  = 100
)

type IPVersion string

const (
	IPAny IPVersion = "any"
	IPv4  IPVersion = "ipv4"
	IPv6  IPVersion = "ipv6"
)

var allowedMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {}, "HEAD": {}, "OPTIONS": {},
}

// Call is one HTTP request of a workflow.
type Call struct {
	ID          uuid.UUID
	Position    int
	Name        string
	Method      string
	URL         string
	Headers     map[string]string
	Query       map[string]string
	Body        string
	Timeout     time.Duration
	VerifyTLS   bool
	IPVersion   IPVersion
	ProxyURL    string
	Extractions []Extraction
	Rules       []rule.Rule
}

// Definition is the user input for a call; IDs are assigned by the workflow.
type Definition struct {
	Name        string
	Method      string
	URL         string
	Headers     map[string]string
	Query       map[string]string
	Body        string
	Timeout     time.Duration
	VerifyTLS   *bool
	IPVersion   IPVersion
	ProxyURL    string
	Extractions []Extraction
	Rules       []rule.Rule
}

func New(def Definition, position int, id uuid.UUID) (Call, error) {
	name := strings.TrimSpace(def.Name)
	fail := func(format string, args ...any) (Call, error) {
		return Call{}, &ValidationError{Call: name, Message: fmt.Sprintf(format, args...)}
	}

	if name == "" || len(name) > MaxNameLength {
		return fail("call name is required (max %d characters)", MaxNameLength)
	}

	method := strings.ToUpper(strings.TrimSpace(def.Method))
	if method == "" {
		method = "GET"
	}
	if _, ok := allowedMethods[method]; !ok {
		return fail("unsupported method %q", def.Method)
	}

	rawURL := strings.TrimSpace(def.URL)
	if err := validateURLTemplate(rawURL); err != nil {
		return fail("%v", err)
	}

	timeout := def.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if timeout < MinTimeout || timeout > MaxTimeout {
		return fail("timeout must be between %s and %s", MinTimeout, MaxTimeout)
	}

	ipVersion := def.IPVersion
	if ipVersion == "" {
		ipVersion = IPAny
	}
	if ipVersion != IPAny && ipVersion != IPv4 && ipVersion != IPv6 {
		return fail("ip version must be any, ipv4 or ipv6")
	}

	proxy := strings.TrimSpace(def.ProxyURL)
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fail("proxy must be an http(s) URL")
		}
	}

	for header := range def.Headers {
		if strings.TrimSpace(header) == "" || strings.ContainsAny(header, " \t:\r\n") {
			return fail("invalid header name %q", header)
		}
	}

	extractions := make([]Extraction, 0, len(def.Extractions))
	for _, e := range def.Extractions {
		normalized, err := normalizeExtraction(e)
		if err != nil {
			return fail("%v", err)
		}
		extractions = append(extractions, normalized)
	}

	rules := make([]rule.Rule, len(def.Rules))
	for i, r := range def.Rules {
		if r.ID == uuid.Nil {
			r.ID = uuid.New()
		}
		rules[i] = r
	}

	verifyTLS := true
	if def.VerifyTLS != nil {
		verifyTLS = *def.VerifyTLS
	}

	return Call{
		ID:          id,
		Position:    position,
		Name:        name,
		Method:      method,
		URL:         rawURL,
		Headers:     copyMap(def.Headers),
		Query:       copyMap(def.Query),
		Body:        def.Body,
		Timeout:     timeout,
		VerifyTLS:   verifyTLS,
		IPVersion:   ipVersion,
		ProxyURL:    proxy,
		Extractions: extractions,
		Rules:       rules,
	}, nil
}

func validateURLTemplate(raw string) error {
	if raw == "" {
		return fmt.Errorf("url is required")
	}
	prefix := variable.LiteralPrefix(raw)
	if prefix != raw {
		// The scheme may come from a variable ({{base_url}}/orders): checked at run time.
		if prefix == "" || strings.HasPrefix(prefix, "http://") || strings.HasPrefix(prefix, "https://") {
			return nil
		}
		return fmt.Errorf("url must start with http:// or https:// or a variable")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("url must be an absolute http(s) URL")
	}
	return nil
}

// templates returns every field of the call that may contain placeholders.
func (c Call) templates() []string {
	out := []string{c.URL, c.Body}
	for _, k := range sortedKeys(c.Headers) {
		out = append(out, c.Headers[k])
	}
	for _, k := range sortedKeys(c.Query) {
		out = append(out, c.Query[k])
	}
	for _, r := range c.Rules {
		out = append(out, r.Value)
	}
	return out
}

// References returns the variables used by the call, in a stable order.
func (c Call) References() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, t := range c.templates() {
		for _, name := range variable.References(t) {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// EffectiveRules returns the declared rules, or the default rule when none.
func (c Call) EffectiveRules() []rule.Rule {
	if len(c.Rules) == 0 {
		return rule.Defaults()
	}
	return c.Rules
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
