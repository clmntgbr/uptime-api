package workflow

import (
	"time"

	domaincall "go-api/internal/domain/call"
)

const (
	// RunBudgetPerCallOverhead covers engine work between HTTP calls (variables, rules).
	RunBudgetPerCallOverhead = 2 * time.Second
	// RunBudgetFixedMargin is a fixed safety margin on top of the sum of call timeouts.
	RunBudgetFixedMargin = 30 * time.Second
	// RunBudgetMin is the shortest lease/deadline budget for a run with calls.
	RunBudgetMin = time.Minute
	// RunBudgetMax caps the budget (20 calls × 60s + overhead + margin).
	RunBudgetMax = 25 * time.Minute
)

// EstimateRunBudget returns how long a run of these calls is allowed to take.
// Formula: sum(call.Timeout) + n*2s + 30s, clamped to [1m, 25m].
func EstimateRunBudget(calls []domaincall.Call) time.Duration {
	if len(calls) == 0 {
		return RunBudgetMin
	}
	var sum time.Duration
	for _, c := range calls {
		t := c.Timeout
		if t <= 0 {
			t = domaincall.DefaultTimeout
		}
		sum += t
	}
	budget := sum + time.Duration(len(calls))*RunBudgetPerCallOverhead + RunBudgetFixedMargin
	if budget < RunBudgetMin {
		return RunBudgetMin
	}
	if budget > RunBudgetMax {
		return RunBudgetMax
	}
	return budget
}
