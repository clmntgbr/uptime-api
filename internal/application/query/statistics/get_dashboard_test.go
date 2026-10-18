package statistics

import (
	"context"
	"testing"
	"time"

	domainstatistics "go-api/internal/domain/statistics"

	"github.com/google/uuid"
)

type stubRepo struct {
	gotUser       uuid.UUID
	gotWorkflow   *uuid.UUID
	gotSince      time.Time
	gotDailySince time.Time
	gotUntilDay   time.Time
	out           domainstatistics.DashboardStatistics
	err           error
}

func (s *stubRepo) Dashboard(
	_ context.Context,
	userID uuid.UUID,
	workflowID *uuid.UUID,
	since, dailySince, untilDay time.Time,
) (domainstatistics.DashboardStatistics, error) {
	s.gotUser = userID
	s.gotWorkflow = workflowID
	s.gotSince = since
	s.gotDailySince = dailySince
	s.gotUntilDay = untilDay
	return s.out, s.err
}

func TestGetDashboardHandler_ComputesWindowAndSuccessRate(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	repo := &stubRepo{out: domainstatistics.DashboardStatistics{
		Runs: domainstatistics.RunCounts{Total: 10, Success: 4},
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
	if repo.gotWorkflow != nil {
		t.Fatalf("expected nil workflow filter, got %v", repo.gotWorkflow)
	}
	if !repo.gotUntilDay.Equal(wantUntil) || !repo.gotDailySince.Equal(wantDailySince) {
		t.Fatalf("unexpected daily window since=%s until=%s", repo.gotDailySince, repo.gotUntilDay)
	}
	if got.WindowHours != 24 || got.SuccessRate != 0.4 {
		t.Fatalf("unexpected statistics %+v", got)
	}
	if got.Runs.Daily == nil || got.Calls.Daily == nil || got.Incidents == nil {
		t.Fatal("daily/incidents must be non-nil empty slices")
	}
}

func TestGetDashboardHandler_PassesWorkflowID(t *testing.T) {
	workflowID := uuid.New()
	repo := &stubRepo{}
	_, err := NewGetDashboardHandler(repo).Handle(context.Background(), GetDashboardQuery{
		UserID: uuid.New(), WorkflowID: &workflowID, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotWorkflow == nil || *repo.gotWorkflow != workflowID {
		t.Fatalf("expected workflow filter %s, got %v", workflowID, repo.gotWorkflow)
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
