package rule

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type Subject string

const (
	SubjectStatus       Subject = "status"
	SubjectResponseTime Subject = "response_time"
	SubjectBody         Subject = "body"
	SubjectJSON         Subject = "json"
	SubjectHeader       Subject = "header"
	SubjectSize Subject = "size"
)

type Operator string

const (
	OpIs            Operator = "is"
	OpIsNot         Operator = "is_not"
	OpIn            Operator = "in"
	OpExceeds       Operator = "exceeds"
	OpBelow         Operator = "below"
	OpContains      Operator = "contains"
	OpNotContains   Operator = "not_contains"
	OpExists        Operator = "exists"
	OpNotExists Operator = "not_exists"
)

var allowedOperators = map[Subject][]Operator{
	SubjectStatus:       {OpIs, OpIsNot, OpIn},
	SubjectResponseTime: {OpExceeds},
	SubjectBody:         {OpContains, OpNotContains},
	SubjectJSON:         {OpIs, OpIsNot, OpExists, OpNotExists, OpExceeds, OpBelow},
	SubjectHeader:       {OpIs, OpContains, OpExists},
	SubjectSize: {OpExceeds},
}

// IsAlertCondition tells how a rule reads. Threshold operators describe the bad
// state ("response time exceeds 800 ms" fails when it does); comparison
// operators describe the expected state ("status is 200" fails when it is not).
func (o Operator) IsAlertCondition() bool {
	switch o {
	case OpExceeds, OpBelow:
		return true
	}
	return false
}

func (o Operator) needsValue() bool {
	switch o {
	case OpExists, OpNotExists:
		return false
	}
	return true
}

// Rule is one readable alert sentence: [subject] [operator] [value].
// Target holds the JSONPath (json) or the header name (header).
// Value is normalized: milliseconds for response time, bytes for size, days
// for certificate expiry. Values may contain `{{name}}` placeholders.
type Rule struct {
	ID       uuid.UUID
	Subject  Subject
	Operator Operator
	Target   string
	Value    string
}

var (
	ErrInvalid = errors.New("invalid rule")

	statusPattern = regexp.MustCompile(`^[1-5]([0-9]{2}|xx)$`)
	durationValue = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(ms|s)?$`)
	sizeValue     = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(b|bytes|kb|ko|mb|mo)?$`)
	numberValue   = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// New validates and normalizes a rule.
func New(subject Subject, operator Operator, target, value string) (Rule, error) {
	ops, ok := allowedOperators[subject]
	if !ok {
		return Rule{}, fmt.Errorf("%w: unknown subject %q", ErrInvalid, subject)
	}
	if !containsOperator(ops, operator) {
		return Rule{}, fmt.Errorf("%w: operator %q is not allowed for subject %q", ErrInvalid, operator, subject)
	}

	target = strings.TrimSpace(target)
	value = strings.TrimSpace(value)

	switch subject {
	case SubjectJSON:
		if !strings.HasPrefix(target, "$") {
			return Rule{}, fmt.Errorf("%w: json rules need a JSONPath starting with $", ErrInvalid)
		}
	case SubjectHeader:
		if target == "" || strings.ContainsAny(target, " \t:") {
			return Rule{}, fmt.Errorf("%w: header rules need a header name", ErrInvalid)
		}
	default:
		target = ""
	}

	if !operator.needsValue() {
		value = ""
	} else if value == "" {
		return Rule{}, fmt.Errorf("%w: %s %s needs a value", ErrInvalid, subject, operator)
	}

	normalized, err := normalizeValue(subject, operator, value)
	if err != nil {
		return Rule{}, err
	}

	return Rule{ID: uuid.New(), Subject: subject, Operator: operator, Target: target, Value: normalized}, nil
}

func normalizeValue(subject Subject, operator Operator, value string) (string, error) {
	if value == "" || variable.HasReferences(value) {
		return value, nil
	}
	switch subject {
	case SubjectStatus:
		codes := []string{value}
		if operator == OpIn {
			codes = splitList(value)
		}
		for i, code := range codes {
			code = strings.ToLower(code)
			if !statusPattern.MatchString(code) {
				return "", fmt.Errorf("%w: %q is not a status code or class (200, 2xx)", ErrInvalid, code)
			}
			codes[i] = code
		}
		return strings.Join(codes, ","), nil
	case SubjectResponseTime:
		ms, err := ParseDurationMS(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(ms, 10), nil
	case SubjectSize:
		bytes, err := ParseSizeBytes(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(bytes, 10), nil
	case SubjectJSON:
		if (operator == OpExceeds || operator == OpBelow) && !numberValue.MatchString(value) {
			return "", fmt.Errorf("%w: %q is not a number", ErrInvalid, value)
		}
	}
	return value, nil
}

// References lists the variables used by the rule value.
func (r Rule) References() []string {
	return variable.References(r.Value)
}

// Text renders the rule as the sentence shown in the interface and the history.
func (r Rule) Text() string {
	switch r.Subject {
	case SubjectStatus:
		switch r.Operator {
		case OpIsNot:
			return "status is not " + r.Value
		case OpIn:
			return "status in " + strings.ReplaceAll(r.Value, ",", ", ")
		default:
			return "status is " + r.Value
		}
	case SubjectResponseTime:
		return "response time exceeds " + withUnit(r.Value, " ms")
	case SubjectBody:
		if r.Operator == OpNotContains {
			return "body does not contain " + r.Value
		}
		return "body contains " + r.Value
	case SubjectJSON:
		prefix := "json " + r.Target + " "
		switch r.Operator {
		case OpIsNot:
			return prefix + "is not " + r.Value
		case OpExists:
			return prefix + "exists"
		case OpNotExists:
			return prefix + "does not exist"
		case OpExceeds:
			return prefix + "exceeds " + r.Value
		case OpBelow:
			return prefix + "is below " + r.Value
		default:
			return prefix + "is " + r.Value
		}
	case SubjectHeader:
		prefix := "header " + r.Target + " "
		switch r.Operator {
		case OpContains:
			return prefix + "contains " + r.Value
		case OpExists:
			return prefix + "exists"
		default:
			return prefix + "is " + r.Value
		}
	case SubjectSize:
		if n, err := strconv.ParseInt(r.Value, 10, 64); err == nil {
			return "size exceeds " + FormatBytes(n)
		}
		return "size exceeds " + r.Value
	}
	return string(r.Subject) + " " + string(r.Operator) + " " + r.Value
}

func withUnit(value, unit string) string {
	if variable.HasReferences(value) {
		return value
	}
	return value + unit
}

// Defaults apply when a call declares none: the call fails unless the
// status is 2xx (network errors already make it `error`).
func Defaults() []Rule {
	return []Rule{{Subject: SubjectStatus, Operator: OpIs, Value: "2xx"}}
}

func containsOperator(ops []Operator, op Operator) bool {
	for _, candidate := range ops {
		if candidate == op {
			return true
		}
	}
	return false
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
