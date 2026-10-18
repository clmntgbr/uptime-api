package stats

import (
	"context"
	"testing"
	"time"

	domainstats "go-api/internal/domain/stats"

	"github.com/google/uuid"
)

type stubRepo struct {
	gotUser       uuid.UUID
	gotSince      time.Time
	gotDailySince time.Time
	gotUntilDay   time.Time
	out           domainstats.DashboardStats
	err           error
}

func (s *stubRepo) Dashboard(
	_ context.Context,
	userID uuid.UUID,
	since, dailySince, untilDay time.Time,
) (domainstats.DashboardStats, error) {
	s.gotUser = userID
	s.gotSince = since
	s.gotDailySince = dailySince
	s.gotUntilDay = untilDay
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
	wantUntil := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	wantDailySince := wantUntil.AddDate(0, -6, 0)
	if repo.gotUser != userID || !repo.gotSince.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("unexpected repo args user=%s since=%s", repo.gotUser, repo.gotSince)
	}
	if !repo.gotUntilDay.Equal(wantUntil) || !repo.gotDailySince.Equal(wantDailySince) {
		t.Fatalf("unexpected daily window since=%s until=%s", repo.gotDailySince, repo.gotUntilDay)
	}
	if got.WindowHours != 24 || got.SuccessRate != 0.4 {
		t.Fatalf("unexpected stats %+v", got)
	}
	if got.Calls.Daily == nil {
		t.Fatal("daily must be non-nil empty slice")
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
