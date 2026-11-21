package feed

import (
	"testing"
	"time"

	"go-api/internal/contract"
	domainrun "go-api/internal/domain/run"

	"github.com/google/uuid"
)

func TestEventsFromRun_AnomaliesOnly(t *testing.T) {
	callID := uuid.New()
	r := &domainrun.Run{
		ID: uuid.New(), WorkflowID: uuid.New(), UserID: uuid.New(),
		Status: domainrun.StatusFailed, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
		Calls: []domainrun.CallRun{
			{
				CallID: &callID, Name: "Checkout", Status: domainrun.CallFailed, HTTPStatus: 502,
				FinishedAt: time.Now().UTC(),
			},
			{
				CallID: &callID, Name: "OK", Status: domainrun.CallSuccess, HTTPStatus: 200,
				FinishedAt: time.Now().UTC(),
			},
			{
				CallID: &callID, Name: "Slow", Status: domainrun.CallSuccess, HTTPStatus: 200,
				Timings: contract.Timings{Total: 1240}, FinishedAt: time.Now().UTC(),
				Rules: []domainrun.RuleRun{{
					Subject: "response_time", Observed: "1240", Expected: "800", Passed: false,
				}},
			},
		},
	}
	evs := EventsFromRun(r)
	kinds := map[Kind]int{}
	for _, e := range evs {
		kinds[e.Kind]++
	}
	if kinds[KindCallFailed] != 1 || kinds[KindCallSlow] != 1 || kinds[KindRunFailed] != 1 {
		t.Fatalf("unexpected kinds %+v", kinds)
	}
	if kinds[KindCallErrored] != 0 {
		t.Fatal("no errored expected")
	}
}

func TestEventsFromRun_TimedOut(t *testing.T) {
	r := domainrun.NewTimedOut(uuid.New(), uuid.New(), uuid.New(), time.Now().UTC().Add(-time.Minute), time.Now().UTC())
	evs := EventsFromRun(r)
	if len(evs) != 1 || evs[0].Kind != KindRunTimedOut {
		t.Fatalf("got %+v", evs)
	}
}

func TestEventsFromRun_Skipped(t *testing.T) {
	r := domainrun.NewSkipped(uuid.New(), uuid.New(), time.Now().UTC())
	evs := EventsFromRun(r)
	if len(evs) != 1 || evs[0].Kind != KindRunSkipped {
		t.Fatalf("got %+v", evs)
	}
}

func TestEventsFromRun_SuccessSilent(t *testing.T) {
	r := &domainrun.Run{
		ID: uuid.New(), WorkflowID: uuid.New(), UserID: uuid.New(),
		Status: domainrun.StatusSuccess, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
		Calls: []domainrun.CallRun{{Name: "A", Status: domainrun.CallSuccess, HTTPStatus: 200}},
	}
	if evs := EventsFromRun(r); len(evs) != 0 {
		t.Fatalf("success must be silent, got %+v", evs)
	}
}
