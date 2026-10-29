package rule

import (
	"fmt"
	"strings"
)

// Parse reads the sentence form used in workflow files, e.g.
// "status is 200", "response time exceeds 800 ms", "json $.items exists",
func Parse(text string) (Rule, error) {
	s := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(s)

	cut := func(prefix string) (string, bool) {
		if strings.HasPrefix(lower, prefix) {
			return s[len(prefix):], true
		}
		return "", false
	}
	invalid := fmt.Errorf("%w: cannot understand %q", ErrInvalid, text)

	switch {
	case strings.HasPrefix(lower, "status "):
		if v, ok := cut("status is not "); ok {
			return New(SubjectStatus, OpIsNot, "", v)
		}
		if v, ok := cut("status is "); ok {
			return New(SubjectStatus, OpIs, "", v)
		}
		if v, ok := cut("status in "); ok {
			return New(SubjectStatus, OpIn, "", v)
		}
	case strings.HasPrefix(lower, "response time "):
		if v, ok := cut("response time exceeds "); ok {
			return New(SubjectResponseTime, OpExceeds, "", v)
		}
	case strings.HasPrefix(lower, "body "):
		if v, ok := cut("body does not contain "); ok {
			return New(SubjectBody, OpNotContains, "", v)
		}
		if v, ok := cut("body contains "); ok {
			return New(SubjectBody, OpContains, "", v)
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
				return New(subject, candidate.operator, target, rest[len(candidate.phrase):])
			}
		}
	case strings.HasPrefix(lower, "size "):
		if v, ok := cut("size exceeds "); ok {
			return New(SubjectSize, OpExceeds, "", v)
		}
	}
	return Rule{}, invalid
}
