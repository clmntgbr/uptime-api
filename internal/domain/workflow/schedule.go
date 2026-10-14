package workflow

import (
	"time"

	"go-api/internal/domain/schedule"
)

// ScheduleType controls how next_run_at is computed and whether the workflow
// fires more than once.
type ScheduleType string

const (
	ScheduleInterval ScheduleType = "interval"
	ScheduleOnce     ScheduleType = "once"
	ScheduleAt       ScheduleType = "at"
)

func (t ScheduleType) IsOneShot() bool {
	return t == ScheduleOnce || t == ScheduleAt
}

// NextRunAt returns the first scheduled fire for the definition at `now`.
func NextRunAt(def Definition, now time.Time) (time.Time, error) {
	now = now.UTC()
	switch def.ScheduleType {
	case ScheduleInterval, "":
		return schedule.NextRun(now, def.Interval), nil
	case ScheduleOnce:
		return schedule.NextRun(now, schedule.Tick), nil
	case ScheduleAt:
		if def.RunAt == nil {
			return time.Time{}, &ValidationError{Message: "runAt is required for schedule type at"}
		}
		at := schedule.AlignAt(*def.RunAt)
		if !at.After(now) {
			return time.Time{}, &ValidationError{Message: "runAt must be a future minute"}
		}
		return at, nil
	default:
		return time.Time{}, &ValidationError{Message: "scheduleType must be interval, once or at"}
	}
}
