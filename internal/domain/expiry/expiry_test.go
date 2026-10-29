package expiry

import (
	"testing"
	"time"
)

func TestNormalizeThresholds(t *testing.T) {
	got, err := NormalizeThresholds([]int{7, 30, 7, 14, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []int{30, 14, 7, 1}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if _, err := NormalizeThresholds(nil); err == nil {
		t.Fatal("empty must fail")
	}
	if _, err := NormalizeThresholds([]int{0}); err == nil {
		t.Fatal("0 must fail")
	}
}

func TestCrossedAndNotifyThreshold(t *testing.T) {
	crossed := CrossedThresholds([]int{30, 14, 7, 1}, 5)
	if len(crossed) != 3 || NotifyThreshold(crossed, 5) != 7 {
		t.Fatalf("got crossed=%v notify=%d", crossed, NotifyThreshold(crossed, 5))
	}
}

func TestItemViewState(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	item := Item{
		ExpiresAt:  now.Add(62 * 24 * time.Hour),
		ObservedAt: now,
	}
	state, days, next := ItemViewState(item, []int{60, 30, 14, 7, 1}, now)
	if state != StateOK || days != 62 || next == nil || *next != 60 {
		t.Fatalf("got state=%s days=%d next=%v", state, days, next)
	}
	item.ExpiresAt = now.Add(5 * 24 * time.Hour)
	state, days, _ = ItemViewState(item, []int{30, 14, 7, 1}, now)
	if state != StateWarning || days != 5 {
		t.Fatalf("got state=%s days=%d", state, days)
	}
	item.ObservedAt = now.Add(-8 * 24 * time.Hour)
	state, _, _ = ItemViewState(item, []int{30}, now)
	if state != StateStale {
		t.Fatalf("got %s want stale", state)
	}
}
