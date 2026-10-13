package workflow

import (
	"errors"
	"fmt"
	"math"
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
	SubjectSize         Subject = "size"
	SubjectCertificate  Subject = "certificate"
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
	OpNotExists     Operator = "not_exists"
	OpExpiresWithin Operator = "expires_within"
	OpChainInvalid  Operator = "chain_invalid"
	OpNameMismatch  Operator = "name_mismatch"
)

var allowedOperators = map[Subject][]Operator{
	SubjectStatus:       {OpIs, OpIsNot, OpIn},
	SubjectResponseTime: {OpExceeds},
	SubjectBody:         {OpContains, OpNotContains},
	SubjectJSON:         {OpIs, OpIsNot, OpExists, OpNotExists, OpExceeds, OpBelow},
	SubjectHeader:       {OpIs, OpContains, OpExists},
	SubjectSize:         {OpExceeds},
	SubjectCertificate:  {OpExpiresWithin, OpChainInvalid, OpNameMismatch},
}

// IsAlertCondition tells how a rule reads. Threshold operators describe the bad
// state ("response time exceeds 800 ms" fails when it does); comparison
// operators describe the expected state ("status is 200" fails when it is not).
func (o Operator) IsAlertCondition() bool {
	switch o {
	case OpExceeds, OpBelow, OpExpiresWithin, OpChainInvalid, OpNameMismatch:
		return true
	}
	return false
}

func (o Operator) needsValue() bool {
	switch o {
	case OpExists, OpNotExists, OpChainInvalid, OpNameMismatch:
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
	ErrInvalidRule = errors.New("invalid rule")

	statusPattern = regexp.MustCompile(`^[1-5]([0-9]{2}|xx)$`)
	durationValue = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(ms|s)?$`)
	sizeValue     = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(b|bytes|kb|ko|mb|mo)?$`)
	daysValue     = regexp.MustCompile(`^(\d+)\s*(days?)?$`)
	numberValue   = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
)

// NewRule validates and normalizes a rule.
func NewRule(subject Subject, operator Operator, target, value string) (Rule, error) {
	ops, ok := allowedOperators[subject]
	if !ok {
		return Rule{}, fmt.Errorf("%w: unknown subject %q", ErrInvalidRule, subject)
	}
	if !containsOperator(ops, operator) {
		return Rule{}, fmt.Errorf("%w: operator %q is not allowed for subject %q", ErrInvalidRule, operator, subject)
	}

	target = strings.TrimSpace(target)
	value = strings.TrimSpace(value)

	switch subject {
	case SubjectJSON:
		if !strings.HasPrefix(target, "$") {
			return Rule{}, fmt.Errorf("%w: json rules need a JSONPath starting with $", ErrInvalidRule)
		}
	case SubjectHeader:
		if target == "" || strings.ContainsAny(target, " \t:") {
			return Rule{}, fmt.Errorf("%w: header rules need a header name", ErrInvalidRule)
		}
	default:
		target = ""
	}

	if !operator.needsValue() {
		value = ""
	} else if value == "" {
		return Rule{}, fmt.Errorf("%w: %s %s needs a value", ErrInvalidRule, subject, operator)
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
				return "", fmt.Errorf("%w: %q is not a status code or class (200, 2xx)", ErrInvalidRule, code)
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
	case SubjectCertificate:
		m := daysValue.FindStringSubmatch(strings.ToLower(value))
		if m == nil {
			return "", fmt.Errorf("%w: %q is not a number of days", ErrInvalidRule, value)
		}
		days, _ := strconv.Atoi(m[1])
		if days <= 0 {
			return "", fmt.Errorf("%w: certificate days must be positive", ErrInvalidRule)
		}
		return strconv.Itoa(days), nil
	case SubjectJSON:
		if (operator == OpExceeds || operator == OpBelow) && !numberValue.MatchString(value) {
			return "", fmt.Errorf("%w: %q is not a number", ErrInvalidRule, value)
		}
	}
	return value, nil
}

// ParseDurationMS reads "800 ms", "800", "2 s" or "1.5s" as milliseconds.
func ParseDurationMS(value string) (int64, error) {
	m := durationValue.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if m == nil {
		return 0, fmt.Errorf("%w: %q is not a duration (800 ms, 2 s)", ErrInvalidRule, value)
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	if m[2] == "s" {
		n *= 1000
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: duration must be positive", ErrInvalidRule)
	}
	return int64(math.Round(n)), nil
}

// ParseSizeBytes reads "512", "100 B", "64 KB" or "1.5 MB" as bytes.
func ParseSizeBytes(value string) (int64, error) {
	m := sizeValue.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if m == nil {
		return 0, fmt.Errorf("%w: %q is not a size (100 B, 64 KB, 1 MB)", ErrInvalidRule, value)
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	switch m[2] {
	case "kb", "ko":
		n *= 1024
	case "mb", "mo":
		n *= 1024 * 1024
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: size must be positive", ErrInvalidRule)
	}
	return int64(math.Round(n)), nil
}

// FormatBytes renders a byte count with the largest exact unit.
func FormatBytes(n int64) string {
	switch {
	case n >= 1024*1024 && n%(1024*1024) == 0:
		return fmt.Sprintf("%d MB", n/(1024*1024))
	case n >= 1024 && n%1024 == 0:
		return fmt.Sprintf("%d KB", n/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
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
	case SubjectCertificate:
		switch r.Operator {
		case OpChainInvalid:
			return "certificate chain is invalid"
		case OpNameMismatch:
			return "certificate name does not match"
		default:
			return "certificate expires in less than " + withUnit(r.Value, " days")
		}
	}
	return string(r.Subject) + " " + string(r.Operator) + " " + r.Value
}

func withUnit(value, unit string) string {
	if variable.HasReferences(value) {
		return value
	}
	return value + unit
}

// ParseRule reads the sentence form used in workflow files, e.g.
// "status is 200", "response time exceeds 800 ms", "json $.items exists",
// "certificate expires in less than 14 days".
func ParseRule(text string) (Rule, error) {
	s := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(s)

	cut := func(prefix string) (string, bool) {
		if strings.HasPrefix(lower, prefix) {
			return s[len(prefix):], true
		}
		return "", false
	}
	invalid := fmt.Errorf("%w: cannot understand %q", ErrInvalidRule, text)

	switch {
	case strings.HasPrefix(lower, "status "):
		if v, ok := cut("status is not "); ok {
			return NewRule(SubjectStatus, OpIsNot, "", v)
		}
		if v, ok := cut("status is "); ok {
			return NewRule(SubjectStatus, OpIs, "", v)
		}
		if v, ok := cut("status in "); ok {
			return NewRule(SubjectStatus, OpIn, "", v)
		}
	case strings.HasPrefix(lower, "response time "):
		if v, ok := cut("response time exceeds "); ok {
			return NewRule(SubjectResponseTime, OpExceeds, "", v)
		}
	case strings.HasPrefix(lower, "body "):
		if v, ok := cut("body does not contain "); ok {
			return NewRule(SubjectBody, OpNotContains, "", v)
		}
		if v, ok := cut("body contains "); ok {
			return NewRule(SubjectBody, OpContains, "", v)
		}
	case strings.HasPrefix(lower, "json "), strings.HasPrefix(lower, "header "):
		subject := SubjectJSON
		if strings.HasPrefix(lower, "header ") {
			subject = SubjectHeader
		}
		parts := strings.SplitN(s, " ", 3)
		if len(parts) < 3 {
			return Rule{}, invalid
		}
		target, rest := parts[1], parts[2]
		restLower := strings.ToLower(rest)
		for _, candidate := range []struct {
			phrase   string
			operator Operator
		}{
			{"is not ", OpIsNot},
			{"is below ", OpBelow},
			{"is ", OpIs},
			{"exceeds ", OpExceeds},
			{"contains ", OpContains},
			{"does not exist", OpNotExists},
			{"exists", OpExists},
		} {
			if strings.HasPrefix(restLower, candidate.phrase) {
				return NewRule(subject, candidate.operator, target, rest[len(candidate.phrase):])
			}
		}
	case strings.HasPrefix(lower, "size "):
		if v, ok := cut("size exceeds "); ok {
			return NewRule(SubjectSize, OpExceeds, "", v)
		}
	case strings.HasPrefix(lower, "certificate "):
		if v, ok := cut("certificate expires in less than "); ok {
			return NewRule(SubjectCertificate, OpExpiresWithin, "", v)
		}
		if lower == "certificate chain is invalid" {
			return NewRule(SubjectCertificate, OpChainInvalid, "", "")
		}
		if lower == "certificate name does not match" {
			return NewRule(SubjectCertificate, OpNameMismatch, "", "")
		}
	}
	return Rule{}, invalid
}

// DefaultRules apply when a call declares none: the call fails unless the
// status is 2xx (network errors already make it `error`).
func DefaultRules() []Rule {
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
