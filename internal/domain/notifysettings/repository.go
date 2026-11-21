package notifysettings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DigestCandidate is a workflow opted into a digest cadence.
type DigestCandidate struct {
	Settings
	WorkflowName string
}

type Repository interface {
	Get(ctx context.Context, workflowID uuid.UUID) (*Settings, error)
	Upsert(ctx context.Context, s Settings) error
	// ListDigestCandidates returns settings with the cadence flag on and non-empty emails.
	ListDigestCandidates(ctx context.Context, cadence DigestCadence) ([]DigestCandidate, error)
}

// DigestStats is the light summary for one workflow over a period.
type DigestStats struct {
	RunsTotal         int64
	RunsSuccess       int64
	RunsFailed        int64
	RunsError         int64
	IncidentsOpened   int64
	IncidentsResolved int64
}

func (s DigestStats) Empty() bool {
	return s.RunsTotal == 0 && s.IncidentsOpened == 0 && s.IncidentsResolved == 0
}

func (s DigestStats) SuccessRate() float64 {
	den := s.RunsSuccess + s.RunsFailed + s.RunsError
	if den == 0 {
		return 0
	}
	return float64(s.RunsSuccess) / float64(den) * 100
}

// DigestStatsRepository aggregates run/incident counters for digests.
type DigestStatsRepository interface {
	Stats(ctx context.Context, workflowID uuid.UUID, from, to time.Time) (DigestStats, error)
}
