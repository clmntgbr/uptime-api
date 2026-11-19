package workflow

import (
	"testing"
	"time"

	domaincall "go-api/internal/domain/call"
)

func TestEstimateRunBudget(t *testing.T) {
	t.Run("one call 30s", func(t *testing.T) {
		calls := []domaincall.Call{{Timeout: 30 * time.Second}}
		// 30s + 2s + 30s = 1m2s
		want := time.Minute + 2*time.Second
		got := EstimateRunBudget(calls)
		if got != want {
			t.Fatalf("got %v want %v", got, want)
		}
	})

	t.Run("zero timeout uses default 30s", func(t *testing.T) {
		calls := []domaincall.Call{{Timeout: 0}}
		want := time.Minute + 2*time.Second
		got := EstimateRunBudget(calls)
		if got != want {
			t.Fatalf("got %v want %v", got, want)
		}
	})

	t.Run("short call clamps to min 1m", func(t *testing.T) {
		calls := []domaincall.Call{{Timeout: time.Second}}
		// 1s + 2s + 30s = 33s → clamp to 1m
		got := EstimateRunBudget(calls)
		if got != RunBudgetMin {
			t.Fatalf("got %v want %v", got, RunBudgetMin)
		}
	})

	t.Run("many slow calls under max", func(t *testing.T) {
		calls := make([]domaincall.Call, 20)
		for i := range calls {
			calls[i].Timeout = 60 * time.Second
		}
		// 20*60s + 20*2s + 30s = 21m10s
		got := EstimateRunBudget(calls)
		want := 21*time.Minute + 10*time.Second
		if got != want {
			t.Fatalf("got %v want %v", got, want)
		}
	})

	t.Run("over max is clamped", func(t *testing.T) {
		// Artificial oversized timeouts should still clamp (domain usually caps at 60s).
		calls := make([]domaincall.Call, 20)
		for i := range calls {
			calls[i].Timeout = 2 * time.Minute
		}
		got := EstimateRunBudget(calls)
		if got != RunBudgetMax {
			t.Fatalf("got %v want %v", got, RunBudgetMax)
		}
	})

	t.Run("empty calls returns min", func(t *testing.T) {
		if got := EstimateRunBudget(nil); got != RunBudgetMin {
			t.Fatalf("got %v want %v", got, RunBudgetMin)
		}
	})

	t.Run("three calls at 60s", func(t *testing.T) {
		calls := []domaincall.Call{
			{Timeout: time.Minute},
			{Timeout: time.Minute},
			{Timeout: time.Minute},
		}
		// 3m + 6s + 30s = 3m36s
		want := 3*time.Minute + 36*time.Second
		if got := EstimateRunBudget(calls); got != want {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}
