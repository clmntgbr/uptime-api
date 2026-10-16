package stats

import (
	"context"
	"testing"
	"time"

	domainstats "go-api/internal/domain/stats"

	"github.com/google/uuid"
)

type stubRepo struct {
	gotUser uuid.UUID
	gotSince time.Time
	out     domainstats.DashboardStats
	err     error
}

func (s *stubRepo) Dashboard(_ context.Context, userID uuid.UUID, since time.Time) (domainstats.DashboardStats, error) {
	s.gotUser = userID
	s.gotSince = since
	return s.out, s.err
}

func TestGetDashboardHandler_ComputesWindowAndSuccessRate(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	repo := &stubRepo{out: domainstats.DashboardStats{
		Runs: domainstats.RunCounts{Total: 10, Success: 4},
	}}
	got, err := NewGetDashboardHandler(repo).Handle(context.Background(), GetDashboardQuery{UserID: userID, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotUser != userID || !repo.gotSince.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("unexpected repo args user=%s since=%s", repo.gotUser, repo.gotSince)
	}
	if got.WindowHours != 24 || got.SuccessRate != 0.4 {
		t.Fatalf("unexpected stats %+v", got)
	}
}

func TestGetDashboardHandler_ZeroRunsSuccessRate(t *testing.T) {
	got, err := NewGetDashboardHandler(&stubRepo{}).Handle(context.Background(), GetDashboardQuery{
		UserID: uuid.New(), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SuccessRate != 0 {
		t.Fatalf("expected 0 success rate, got %v", got.SuccessRate)
	}
}
