package schedule

import (
	"time"

	"github.com/google/uuid"
)

// Tick is the scheduler granularity: runs start on whole minutes (hh:mm:00).
const Tick = time.Minute

type Status string

const (
	StatusIdle       Status = "idle"
	StatusRunning    Status = "running"
	StatusCancelling Status = "cancelling"
)

// State is the scheduling state of a workflow. The lease guarantees that a
// workflow never runs twice in parallel.
type State struct {
	WorkflowID           uuid.UUID
	Status               Status
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

// Never is a sentinel next_run_at used after a one-shot (once/at) dispatch so
// the scheduler never picks the workflow again until the user reschedules it.
var Never = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

func New(workflowID uuid.UUID, nextRunAt time.Time) State {
	return State{
		WorkflowID: workflowID,
		Status:     StatusIdle,
		NextRunAt:  nextRunAt.UTC(),
	}
}

// NextRun returns the first boundary strictly after `after` aligned on the
// interval in UTC: a 5 minute workflow runs at hh:00, hh:05, hh:10...
func NextRun(after time.Time, interval time.Duration) time.Time {
	if interval < Tick {
		interval = Tick
	}
	return after.UTC().Truncate(interval).Add(interval)
}

// AlignAt truncates runAt to the minute grid. The caller must reject past values.
func AlignAt(runAt time.Time) time.Time {
	return runAt.UTC().Truncate(Tick)
}

// Reschedule moves the next run to nextRunAt (already computed by the caller).
func (s *State) Reschedule(nextRunAt time.Time) {
	s.NextRunAt = nextRunAt.UTC()
}

// ClearSchedule parks the workflow so LockDue never selects it again.
func (s *State) ClearSchedule() {
	s.NextRunAt = Never
}

// Running reports whether a lease is held (valid or expired), including while cancelling.
func (s State) Running() bool {
	return (s.Status == StatusRunning || s.Status == StatusCancelling) && s.LeaseRunID != nil
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

// RequestCancel moves a running lease to cancelling so remaining calls are skipped.
// ok is true when a lease can be cancelled; transitioned is true only when this
// call flipped running→cancelling (already-cancelling returns ok without transition).
func (s *State) RequestCancel(now time.Time) (runID uuid.UUID, transitioned, ok bool) {
	if !s.LeaseActive(now) || s.LeaseRunID == nil {
		return uuid.Nil, false, false
	}
	id := *s.LeaseRunID
	if s.Status == StatusCancelling {
		return id, false, true
	}
	s.Status = StatusCancelling
	return id, true, true
}

// TakeOverCancelling drops an active cancelling lease so a new run can start.
// The previous run keeps its cancel signal; callers must finalize that run
// (MarkProcessed + record) so a late Result cannot overwrite state.
func (s *State) TakeOverCancelling(now time.Time) (ExpiredLease, bool) {
	if !s.LeaseActive(now) || s.Status != StatusCancelling || s.LeaseRunID == nil {
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
	return lease, true
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
// still belongs to that run: a late result never frees a newer run's lease,
// nor does it overwrite last_* after a cancelling takeover.
func (s *State) Complete(runID uuid.UUID, status string, finishedAt time.Time) {
	if s.Running() && s.LeaseRunID != nil && *s.LeaseRunID != runID {
		return
	}
	if s.Running() && *s.LeaseRunID == runID {
		s.release()
	}
	if s.LastRunAt != nil && s.LastRunAt.After(finishedAt) {
		return
	}
	s.recordOutcome(runID, status, finishedAt)
}

// RevertCancel restores a cancelling lease to running when the cancel signal
// could not be published (so stop can be retried).
func (s *State) RevertCancel(runID uuid.UUID) bool {
	if s.Status != StatusCancelling || s.LeaseRunID == nil || *s.LeaseRunID != runID {
		return false
	}
	s.Status = StatusRunning
	return true
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
