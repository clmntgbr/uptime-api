package schedule

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAlignAt(t *testing.T) {
	raw := time.Date(2026, 10, 13, 14, 3, 45, 0, time.UTC)
	want := time.Date(2026, 10, 13, 14, 3, 0, 0, time.UTC)
	if got := AlignAt(raw); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNextRun_AlignedOnInterval(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 7, 42, 0, time.UTC)
	cases := map[time.Duration]time.Time{
		time.Minute:      time.Date(2026, 10, 13, 10, 8, 0, 0, time.UTC),
		5 * time.Minute:  time.Date(2026, 10, 13, 10, 10, 0, 0, time.UTC),
		15 * time.Minute: time.Date(2026, 10, 13, 10, 15, 0, 0, time.UTC),
		time.Hour:        time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC),
	}
	for interval, want := range cases {
		if got := NextRun(now, interval); !got.Equal(want) {
			t.Errorf("interval %s: got %s want %s", interval, got, want)
		}
	}
	boundary := time.Date(2026, 10, 13, 10, 10, 0, 0, time.UTC)
	if got := NextRun(boundary, 5*time.Minute); !got.Equal(boundary.Add(5 * time.Minute)) {
		t.Errorf("a boundary must move to the next one, got %s", got)
	}
}

func TestState_LeaseLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	s := New(uuid.New(), NextRun(now, time.Minute))
	runID := uuid.New()
	s.Lease(runID, now, 6*time.Minute)

	if !s.LeaseActive(now.Add(5 * time.Minute)) {
		t.Fatal("lease must be active before it expires")
	}
	if s.LeaseActive(now.Add(7 * time.Minute)) {
		t.Fatal("lease must expire")
	}

	other := uuid.New()
	s.Complete(other, "success", now.Add(time.Minute))
	if !s.Running() {
		t.Fatal("a result for another run must not release the lease")
	}
	if s.LastStatus == "success" {
		t.Fatal("a late result for another run must not record an outcome")
	}

	s.Complete(runID, "failed", now.Add(2*time.Minute))
	if s.Running() || s.ConsecutiveFailures != 1 || s.LastStatus != "failed" {
		t.Fatalf("unexpected state %+v", s)
	}

	s.Lease(uuid.New(), now.Add(3*time.Minute), 6*time.Minute)
	lease, ok := s.ExpireLease()
	if !ok || lease.RunID == uuid.Nil || s.Running() || s.ConsecutiveFailures != 2 {
		t.Fatalf("expired lease must count as an error: %+v", s)
	}
}

func TestState_RequestCancelAndTakeover(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	s := New(uuid.New(), NextRun(now, time.Minute))
	oldRun := uuid.New()
	s.Lease(oldRun, now, 6*time.Minute)

	id, transitioned, ok := s.RequestCancel(now)
	if !ok || !transitioned || id != oldRun || s.Status != StatusCancelling || !s.LeaseActive(now) {
		t.Fatalf("cancel must keep an active cancelling lease: %+v", s)
	}
	id, transitioned, ok = s.RequestCancel(now)
	if !ok || transitioned || id != oldRun {
		t.Fatal("second cancel must be idempotent without transition")
	}

	old, took := s.TakeOverCancelling(now)
	if !took || old.RunID != oldRun || s.Running() {
		t.Fatalf("takeover must drop the cancelling lease: %+v", s)
	}
	newRun := uuid.New()
	s.Lease(newRun, now.Add(time.Second), 6*time.Minute)

	s.Complete(oldRun, "failed", now.Add(2*time.Second))
	if !s.Running() || *s.LeaseRunID != newRun || s.LastStatus == "failed" || s.ConsecutiveFailures != 0 {
		t.Fatalf("late cancelled result must not touch the new lease: %+v", s)
	}

	s.Complete(newRun, "success", now.Add(3*time.Second))
	if s.Running() || s.LastStatus != "success" {
		t.Fatalf("new run must complete normally: %+v", s)
	}
}

func TestState_RevertCancel(t *testing.T) {
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	s := New(uuid.New(), NextRun(now, time.Minute))
	runID := uuid.New()
	s.Lease(runID, now, 6*time.Minute)
	s.RequestCancel(now)

	if !s.RevertCancel(runID) || s.Status != StatusRunning {
		t.Fatalf("revert must restore running: %+v", s)
	}
	if s.RevertCancel(runID) {
		t.Fatal("revert must be a no-op when not cancelling")
	}
}
