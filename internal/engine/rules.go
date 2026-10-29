package engine

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-api/internal/contract"
	domainrule "go-api/internal/domain/rule"
)

type observation struct {
	status  int
	totalMS int64
	size    int64
	headers http.Header
	body    *body
	tls     *contract.TLSInfo
	now     time.Time
}

// evaluate checks one rule. Comparison operators (is, contains, exists...) pass
// when the sentence is true; threshold operators (exceeds, below, certificate
// alerts) describe the bad state and fail when the sentence is true.
func evaluate(rule domainrule.Rule, obs observation) contract.RuleRun {
	res := contract.RuleRun{
		Subject:  string(rule.Subject),
		Operator: string(rule.Operator),
		Text:     rule.Text(),
	}
	pass := func(observed, expected string, ok bool) contract.RuleRun {
		res.Observed, res.Expected, res.Passed = observed, expected, ok
		return res
	}

	switch rule.Subject {
	case domainrule.SubjectStatus:
		observed := strconv.Itoa(obs.status)
		codes := strings.Split(rule.Value, ",")
		matched := false
		for _, code := range codes {
			if statusMatches(obs.status, strings.TrimSpace(code)) {
				matched = true
				break
			}
		}
		switch rule.Operator {
		case domainrule.OpIsNot:
			return pass(observed, "not "+rule.Value, !matched)
		case domainrule.OpIn:
			return pass(observed, "one of "+strings.ReplaceAll(rule.Value, ",", ", "), matched)
		default:
			return pass(observed, rule.Value, matched)
		}

	case domainrule.SubjectResponseTime:
		limit, err := strconv.ParseInt(rule.Value, 10, 64)
		if err != nil {
			if limit, err = domainrule.ParseDurationMS(rule.Value); err != nil {
				return pass(formatMS(obs.totalMS), rule.Value, false)
			}
		}
		return pass(formatMS(obs.totalMS), "at most "+formatMS(limit), obs.totalMS <= limit)

	case domainrule.SubjectSize:
		limit, err := strconv.ParseInt(rule.Value, 10, 64)
		if err != nil {
			if limit, err = domainrule.ParseSizeBytes(rule.Value); err != nil {
				return pass(domainrule.FormatBytes(obs.size), rule.Value, false)
			}
		}
		return pass(domainrule.FormatBytes(obs.size), "at most "+domainrule.FormatBytes(limit), obs.size <= limit)

	case domainrule.SubjectBody:
		contains := strings.Contains(string(obs.body.raw), rule.Value)
		if rule.Operator == domainrule.OpNotContains {
			return pass(presence(contains), "absent", !contains)
		}
		return pass(presence(contains), "present", contains)

	case domainrule.SubjectHeader:
		values, exists := obs.headers[http.CanonicalHeaderKey(rule.Target)]
		value := strings.Join(values, ", ")
		switch rule.Operator {
		case domainrule.OpExists:
			return pass(presence(exists), "present", exists)
		case domainrule.OpContains:
			return pass(observedOrAbsent(value, exists), "contains "+rule.Value, exists && strings.Contains(value, rule.Value))
		default:
			return pass(observedOrAbsent(value, exists), rule.Value, exists && value == rule.Value)
		}

	case domainrule.SubjectJSON:
		return evaluateJSON(rule, obs, pass)
	}
	return pass("", rule.Value, false)
}

func evaluateJSON(rule domainrule.Rule, obs observation, pass func(string, string, bool) contract.RuleRun) contract.RuleRun {
	v, err := obs.body.jsonPath(rule.Target)
	found := err == nil
	observed := "absent"
	if found {
		observed = stringify(v)
	} else if !errors.Is(err, errNotFound) {
		observed = err.Error()
	}

	switch rule.Operator {
	case domainrule.OpExists:
		return pass(observed, "present", found)
	case domainrule.OpNotExists:
		return pass(observed, "absent", !found)
	case domainrule.OpIsNot:
		return pass(observed, "not "+rule.Value, !found || observed != rule.Value)
	case domainrule.OpExceeds, domainrule.OpBelow:
		limit, limitErr := strconv.ParseFloat(rule.Value, 64)
		n, numErr := toNumber(v)
		if !found || numErr != nil || limitErr != nil {
			return pass(observed, "a number", false)
		}
		if rule.Operator == domainrule.OpExceeds {
			return pass(observed, "at most "+rule.Value, n <= limit)
		}
		return pass(observed, "at least "+rule.Value, n >= limit)
	default:
		return pass(observed, rule.Value, found && observed == rule.Value)
	}
}

func statusMatches(status int, code string) bool {
	code = strings.ToLower(code)
	if len(code) == 3 && strings.HasSuffix(code, "xx") {
		return status/100 == int(code[0]-'0')
	}
	n, err := strconv.Atoi(code)
	return err == nil && n == status
}

func toNumber(v any) (float64, error) {
	switch t := v.(type) {
	case int64:
		return float64(t), nil
	case float64:
		return t, nil
	case string:
		return strconv.ParseFloat(t, 64)
	}
	return 0, errors.New("not a number")
}

func formatMS(ms int64) string { return strconv.FormatInt(ms, 10) + " ms" }

func presence(ok bool) string {
	if ok {
		return "present"
	}
	return "absent"
}

func observedOrAbsent(value string, exists bool) string {
	if !exists {
		return "absent"
	}
	return value
}
