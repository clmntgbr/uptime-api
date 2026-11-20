package expiry

import (
	"testing"
	"time"
)

func TestScanWindow_CoversEveryNotifiableExpiry(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	from, to := ScanWindow(now)

	// Items past ExpiredAlertDays and items beyond the largest threshold can
	// never produce a notification: they must sit outside the window.
	if !from.Before(now.AddDate(0, 0, -ExpiredAlertDays)) {
		t.Fatalf("window start %v must reach ExpiredAlertDays back", from)
	}
	if !to.After(now.AddDate(0, 0, MaxThresholdDays)) {
		t.Fatalf("window end %v must reach MaxThresholdDays ahead", to)
	}
	tooOld := now.AddDate(0, 0, -(ExpiredAlertDays + 10))
	tooFar := now.AddDate(0, 0, MaxThresholdDays+10)
	if !tooOld.Before(from) || !tooFar.After(to) {
		t.Fatal("window must exclude items that cannot notify")
	}
}
