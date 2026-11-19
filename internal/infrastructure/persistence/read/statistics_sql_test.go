package read

import (
	"regexp"
	"strings"
	"testing"
)

// danglingAnd matches an AND left without a condition, e.g. "AND AND x" or
// "AND" directly before GROUP BY / ORDER BY / a closing paren. Such SQL only
// fails at runtime, and only for the mode that renders no workflow filter.
var danglingAnd = regexp.MustCompile(`(?i)\bAND\s+(AND\b|GROUP\b|ORDER\b|\)|LIMIT\b|$)`)

func TestStatisticsSQL_RendersValidInBothModes(t *testing.T) {
	queries := map[string]string{
		"workflowCounts":     workflowCountsSQL,
		"runCounts":          runCountsSQL,
		"latencyAvg":         latencyAvgSQL,
		"runDaily":           runDailySQL,
		"runHourly":          runHourlySQL,
		"callCounts":         callCountsSQL,
		"callDaily":          callDailySQL,
		"callDuration":       callDurationSQL,
		"callDurationHourly": callDurationHourlySQL,
	}
	for name, q := range queries {
		for _, filtered := range []bool{false, true} {
			out := renderWorkflowFilter(q, filtered)
			if strings.Contains(out, "{{") {
				t.Fatalf("%s (filtered=%v): placeholder not rendered", name, filtered)
			}
			if danglingAnd.MatchString(out) {
				t.Fatalf("%s (filtered=%v): dangling AND in rendered SQL:\n%s", name, filtered, out)
			}
			if filtered && !strings.Contains(out, "workflow_id = @workflow_id") && !strings.Contains(out, "id = @workflow_id") {
				t.Fatalf("%s: workflow filter missing when filtered", name)
			}
			if !filtered && strings.Contains(out, "@workflow_id") {
				t.Fatalf("%s: workflow filter leaked when not filtered", name)
			}
		}
	}
}
