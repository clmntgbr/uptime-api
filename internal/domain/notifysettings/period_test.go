package notifysettings

import (
	"testing"
	"time"
)

func TestPeriodForDaily(t *testing.T) {
	now := time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC) // Wed
	from, to := PeriodFor(DigestDaily, now)
	if !from.Equal(time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)) ||
		!to.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("daily got %v %v", from, to)
	}
}

func TestPeriodForWeekly(t *testing.T) {
	// Monday 17 Nov 2026 08:00 → previous Mon 10 → Sun 16
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)
	from, to := PeriodFor(DigestWeekly, now)
	wantFrom := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("weekly got %v %v", from, to)
	}
}

func TestPeriodForMonthly(t *testing.T) {
	now := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)
	from, to := PeriodFor(DigestMonthly, now)
	wantFrom := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	if !from.Equal(wantFrom) || !to.Equal(wantTo) {
		t.Fatalf("monthly got %v %v", from, to)
	}
}

func TestDueCadences(t *testing.T) {
	mon := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC) // Monday 1st? Nov 2 2026 is Monday
	if mon.Weekday() != time.Monday || mon.Day() != 2 {
		t.Fatalf("fixture: %v weekday=%v day=%d", mon, mon.Weekday(), mon.Day())
	}
	got := DueCadences(mon)
	if len(got) != 2 || got[0] != DigestDaily || got[1] != DigestWeekly {
		t.Fatalf("monday got %#v", got)
	}
	first := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC) // Tuesday
	got = DueCadences(first)
	if len(got) != 2 || got[0] != DigestDaily || got[1] != DigestMonthly {
		t.Fatalf("1st got %#v", got)
	}
}
