package notifysettings

import "time"

// PeriodFor returns the UTC half-open window [from, to) covered by a digest
// sent at `now` (typically 08:00 UTC on the send morning).
func PeriodFor(cadence DigestCadence, now time.Time) (from, to time.Time) {
	now = now.UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch cadence {
	case DigestDaily:
		to = day
		from = day.AddDate(0, 0, -1)
	case DigestWeekly:
		// Monday 00:00 of the current week is `to`; previous Mon→Sun is the window.
		weekday := int(day.Weekday()) // Sunday=0
		if weekday == 0 {
			weekday = 7
		}
		to = day.AddDate(0, 0, -(weekday - 1)) // this Monday
		from = to.AddDate(0, 0, -7)
	case DigestMonthly:
		to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		from = to.AddDate(0, -1, 0)
	default:
		to = day
		from = day
	}
	return from, to
}

// DueCadences lists cadences that should send at this UTC instant (hour/minute ignored).
func DueCadences(now time.Time) []DigestCadence {
	now = now.UTC()
	out := []DigestCadence{DigestDaily}
	if now.Weekday() == time.Monday {
		out = append(out, DigestWeekly)
	}
	if now.Day() == 1 {
		out = append(out, DigestMonthly)
	}
	return out
}
