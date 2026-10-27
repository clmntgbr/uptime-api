package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"go-api/internal/contract"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

var errNotFound = errors.New("not found")

// body lazily parses the response JSON once for every extraction and rule.
type body struct {
	raw    []byte
	parsed any
	err    error
	done   bool
}

func (b *body) json() (any, error) {
	if !b.done {
		b.done = true
		b.parsed, b.err = oj.Parse(b.raw)
		if b.err != nil {
			b.err = errors.New("response is not JSON")
		}
	}
	return b.parsed, b.err
}

// jsonPath returns the first value at path, or errNotFound.
func (b *body) jsonPath(path string) (any, error) {
	doc, err := b.json()
	if err != nil {
		return nil, err
	}
	expr, err := jp.ParseString(path)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath %q", path)
	}
	results := expr.Get(doc)
	if len(results) == 0 {
		return nil, errNotFound
	}
	return results[0], nil
}

func extract(e contract.JobExtraction, resp *Response, b *body) (string, error) {
	switch e.Source {
	case "status":
		return strconv.Itoa(resp.Status), nil
	case "header":
		value := resp.Headers.Get(e.Expression)
		if value == "" {
			return "", fmt.Errorf("header %s is absent", e.Expression)
		}
		return clampExtractionValue(value), nil
	case "regex":
		re, err := regexp.Compile(e.Expression)
		if err != nil {
			return "", fmt.Errorf("invalid regular expression")
		}
		haystack := b.raw
		if len(haystack) > MaxRegexBodyBytes {
			haystack = haystack[:MaxRegexBodyBytes]
		}
		m := re.FindSubmatch(haystack)
		if m == nil {
			return "", fmt.Errorf("regular expression does not match")
		}
		if len(m) > 1 {
			return clampExtractionValue(string(m[1])), nil
		}
		return clampExtractionValue(string(m[0])), nil
	case "json":
		v, err := b.jsonPath(e.Expression)
		if errors.Is(err, errNotFound) {
			return "", fmt.Errorf("%s not found in response", e.Expression)
		}
		if err != nil {
			return "", err
		}
		return clampExtractionValue(stringify(v)), nil
	}
	return "", fmt.Errorf("unknown extraction source %q", e.Source)
}

func clampExtractionValue(v string) string {
	if len(v) <= MaxExtractionValueBytes {
		return v
	}
	return v[:MaxExtractionValueBytes]
}

// stringify renders a JSON value as it is used in a template: strings without
// quotes, other values as JSON.
func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(raw)
	}
}
