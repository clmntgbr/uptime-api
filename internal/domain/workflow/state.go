package workflow

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleStatus string

const (
	StatusIdle    ScheduleStatus = "idle"
	StatusRunning ScheduleStatus = "running"
)

// State is the scheduling state of a workflow. The lease guarantees that a
// workflow never runs twice in parallel.
type State struct {
	WorkflowID           uuid.UUID
	Status               ScheduleStatus
	NextRunAt            time.Time
	LeaseRunID           *uuid.UUID
	LeaseStartedAt       *time.Time
	LeaseUntil           *time.Time
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastRunID            *uuid.UUID
	LastRunAt            *time.Time
	LastStatus           string
}

func NewState(workflowID uuid.UUID, now time.Time, interval time.Duration) State {
	return State{
		WorkflowID: workflowID,
		Status:     StatusIdle,
		NextRunAt:  NextRun(now, interval),
	}
}

// NextRun returns the first boundary strictly after `after` aligned on the
// interval in UTC: a 5 minute workflow runs at hh:00, hh:05, hh:10...
func NextRun(after time.Time, interval time.Duration) time.Time {
	if interval < MinInterval {
		interval = MinInterval
	}
	return after.UTC().Truncate(interval).Add(interval)
}

// Running reports whether a lease is held (valid or expired).
func (s State) Running() bool {
	return s.Status == StatusRunning && s.LeaseRunID != nil
}

func (s State) LeaseActive(now time.Time) bool {
	return s.Running() && s.LeaseUntil != nil && s.LeaseUntil.After(now)
}

// Lease marks the workflow as running for runID until now+duration.
func (s *State) Lease(runID uuid.UUID, now time.Time, duration time.Duration) {
	until := now.Add(duration)
	started := now
	s.Status = StatusRunning
	s.LeaseRunID = &runID
	s.LeaseStartedAt = &started
	s.LeaseUntil = &until
}

// ExpiredLease is what remains of a lease nobody completed in time.
type ExpiredLease struct {
	RunID     uuid.UUID
	StartedAt time.Time
	Until     time.Time
}

// ExpireLease releases an expired lease and returns it, so the scheduler can
// record the run as `error` (timeout) before the next pass.
func (s *State) ExpireLease() (ExpiredLease, bool) {
	if !s.Running() {
		return ExpiredLease{}, false
	}
	lease := ExpiredLease{RunID: *s.LeaseRunID}
	if s.LeaseStartedAt != nil {
		lease.StartedAt = *s.LeaseStartedAt
	}
	if s.LeaseUntil != nil {
		lease.Until = *s.LeaseUntil
	}
	s.release()
	s.recordOutcome(lease.RunID, "error", lease.Until)
	return lease, true
}

// ReleaseLease drops the lease of runID without recording an outcome (dispatch failed).
func (s *State) ReleaseLease(runID uuid.UUID) bool {
	if !s.Running() || *s.LeaseRunID != runID {
		return false
	}
	s.release()
	return true
}

// Complete records the outcome of runID. The lease is only released when it
// still belongs to that run: a late result never frees a newer run's lease.
func (s *State) Complete(runID uuid.UUID, status string, finishedAt time.Time) {
	if s.Running() && *s.LeaseRunID == runID {
		s.release()
	}
	if s.LastRunAt != nil && s.LastRunAt.After(finishedAt) {
		return
	}
	s.recordOutcome(runID, status, finishedAt)
}

func (s *State) release() {
	s.Status = StatusIdle
	s.LeaseRunID = nil
	s.LeaseStartedAt = nil
	s.LeaseUntil = nil
}

func (s *State) recordOutcome(runID uuid.UUID, status string, at time.Time) {
	switch status {
	case "success":
		s.ConsecutiveSuccesses++
		s.ConsecutiveFailures = 0
	case "failed", "error":
		s.ConsecutiveFailures++
		s.ConsecutiveSuccesses = 0
	default:
		return
	}
	id := runID
	when := at
	s.LastRunID = &id
	s.LastRunAt = &when
	s.LastStatus = status
}
