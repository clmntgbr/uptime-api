package incident

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFingerprint(t *testing.T) {
	wf := uuid.MustParse("01960000-0000-7000-8000-000000000001")
	call := uuid.MustParse("01960000-0000-7000-8000-000000000002")
	got := Fingerprint(wf, KindCallFailed, CallSubject(call))
	want := wf.String() + ":call_failed:" + call.String()
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	empty := Fingerprint(wf, KindWorkflowFailed, "")
	if empty != wf.String()+":workflow_failed:"+uuid.Nil.String() {
		t.Fatalf("empty subject: %q", empty)
	}
}

func TestNextCounters(t *testing.T) {
	f, s := NextCounters(true, 2, 5)
	if f != 3 || s != 0 {
		t.Fatalf("failing: got %d %d", f, s)
	}
	f, s = NextCounters(false, 3, 1)
	if f != 0 || s != 2 {
		t.Fatalf("healthy: got %d %d", f, s)
	}
}

func TestShouldSkipSignal(t *testing.T) {
	run := uuid.New()
	at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	state := SignalState{LastRunID: &run, LastSignalAt: &at}
	if !ShouldSkipSignal(state, &run, at.Add(time.Minute)) {
		t.Fatal("same run id must skip")
	}
	other := uuid.New()
	if !ShouldSkipSignal(state, &other, at) {
		t.Fatal("equal observed_at must skip")
	}
	if ShouldSkipSignal(state, &other, at.Add(time.Second)) {
		t.Fatal("newer observation must apply")
	}
}

func TestShouldReopen(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	resolved := now.Add(-10 * time.Minute)
	if !ShouldReopen(&resolved, now) {
		t.Fatal("within 30m must reopen")
	}
	old := now.Add(-31 * time.Minute)
	if ShouldReopen(&old, now) {
		t.Fatal("after 30m must not reopen")
	}
}

func TestApplyReopenResetsReminders(t *testing.T) {
	now := time.Now().UTC()
	resolved := now.Add(-5 * time.Minute)
	rem := now.Add(-time.Hour)
	inc := Incident{
		State: StateResolved, ResolvedAt: &resolved,
		ReminderCount: 2, LastReminderAt: &rem,
		AcknowledgedBy: "u", AcknowledgedAt: &resolved,
	}
	run := uuid.New()
	inc.ApplyReopen(now, map[string]any{"x": 1}, &run, SeverityCritical)
	if inc.State != StateOpen || inc.ReopenCount != 1 || inc.ReminderCount != 0 ||
		inc.LastReminderAt != nil || inc.AcknowledgedBy != "" || inc.ResolvedAt != nil {
		t.Fatalf("unexpected reopen state %+v", inc)
	}
}

func TestDefaultPolicyAvailability(t *testing.T) {
	p := DefaultPolicy(uuid.New(), uuid.New(), KindAvailabilityLow)
	if p.Enabled || p.OpenAfter != 1 || p.CloseAfter != DefaultCloseAfter {
		t.Fatalf("unexpected %+v", p)
	}
	th, rec := p.AvailabilityThresholds()
	if th != 99 || rec != 99.5 {
		t.Fatalf("thresholds %v %v", th, rec)
	}
}

func TestValidatePolicyRecovery(t *testing.T) {
	err := ValidatePolicyFields(true, SeverityWarning, 1, 1, KindAvailabilityLow, map[string]any{
		"threshold": 99.0, "recoveryThreshold": 98.0,
	})
	if err == nil {
		t.Fatal("recovery < threshold must fail")
	}
}

func TestTLSSignal(t *testing.T) {
	now := time.Now().UTC()
	thresholds := []int{30, 14, 7}
	if sig := TLSSignal("a.com:443", 40, thresholds, now); sig == nil || sig.Failing {
		t.Fatalf("above all thresholds must be healthy: %+v", sig)
	}
	// Renewal often lands between higher thresholds; daysLeft > last must close.
	if sig := TLSSignal("a.com:443", 20, thresholds, now); sig == nil || sig.Failing {
		t.Fatalf("above last threshold must be healthy (renewal): %+v", sig)
	}
	if sig := TLSSignal("a.com:443", 5, thresholds, now); sig == nil || !sig.Failing {
		t.Fatalf("at/below last must fail: %+v", sig)
	}
	if sig := TLSSignal("a.com:443", -1, thresholds, now); sig == nil || !sig.Failing ||
		sig.Severity == nil || *sig.Severity != SeverityCritical {
		t.Fatalf("expired must be critical failing: %+v", sig)
	}
}

func TestAvailabilitySignalHysteresis(t *testing.T) {
	now := time.Now().UTC()
	if sig := AvailabilitySignal(98.0, 99.0, 99.5, now); sig == nil || !sig.Failing {
		t.Fatal("below threshold must fail")
	}
	if sig := AvailabilitySignal(99.2, 99.0, 99.5, now); sig != nil {
		t.Fatal("between threshold and recovery must be nil")
	}
	if sig := AvailabilitySignal(99.6, 99.0, 99.5, now); sig == nil || sig.Failing {
		t.Fatal("at recovery must be healthy")
	}
}

func TestShouldSuppressNoRun(t *testing.T) {
	if ShouldSuppressNoRun(4, 10, NoRunSuppressMinCount, NoRunSuppressMinRatio) {
		t.Fatal("below min count must not suppress")
	}
	if !ShouldSuppressNoRun(5, 10, NoRunSuppressMinCount, NoRunSuppressMinRatio) {
		t.Fatal("5 of 10 must suppress")
	}
	if ShouldSuppressNoRun(5, 20, NoRunSuppressMinCount, NoRunSuppressMinRatio) {
		t.Fatal("5 of 20 is under 50%")
	}
}

func TestSilenceAnchorIgnoresDowntimeWhileDisabled(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	oldRun := time.Date(2026, 11, 4, 21, 0, 0, 0, time.UTC)
	enabledAt := time.Date(2026, 11, 5, 13, 0, 0, 0, time.UTC)
	wf := EnabledWorkflow{
		CreatedAt: created, EnabledAt: enabledAt, LastRunAt: &oldRun,
	}
	got := wf.SilenceAnchor()
	if !got.Equal(enabledAt) {
		t.Fatalf("anchor=%s want enabledAt=%s", got, enabledAt)
	}
	laterRun := enabledAt.Add(2 * time.Minute)
	wf.LastRunAt = &laterRun
	if got := wf.SilenceAnchor(); !got.Equal(laterRun) {
		t.Fatalf("after a run, anchor=%s want lastRun=%s", got, laterRun)
	}
}
