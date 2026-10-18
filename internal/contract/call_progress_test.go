package contract

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCallProgress_Validate(t *testing.T) {
	id := uuid.NewString()
	ok := CallProgress{
		V: CallProgressVersion, ID: id, Type: CallProgressType,
		Phase: CallProgressPhaseFinished,
		UserID: id, WorkflowID: id, RunID: id, Trigger: TriggerSchedule,
		Call: CallRun{Name: "login", Order: 1, Status: CallStatusSuccess, CallID: id},
		EmittedAt: time.Now().UTC(),
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}

	started := CallProgress{
		V: CallProgressVersion, ID: id, Type: CallProgressType,
		Phase: CallProgressPhaseStarted,
		UserID: id, WorkflowID: id, RunID: id, Trigger: TriggerSchedule,
		Call: CallRun{Name: "login", Order: 1, CallID: id},
		EmittedAt: time.Now().UTC(),
	}
	if err := started.Validate(); err != nil {
		t.Fatal(err)
	}

	legacy := ok
	legacy.Phase = ""
	if err := legacy.Validate(); err != nil {
		t.Fatal("empty phase must default to finished")
	}

	bad := ok
	bad.Call.Order = 0
	if err := bad.Validate(); err == nil {
		t.Fatal("order 0 must fail")
	}

	badPath := ok
	badPath.Type = ResultType
	if err := badPath.Validate(); err == nil {
		t.Fatal("wrong type must fail")
	}

	badPhase := ok
	badPhase.Phase = "running"
	if err := badPhase.Validate(); err == nil {
		t.Fatal("unknown phase must fail")
	}
}
