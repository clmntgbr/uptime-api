package kafka

import "testing"

func TestPartitionState_CommitsUpToOldestInFlight(t *testing.T) {
	st := newPartitionState()
	for offset := int64(10); offset <= 14; offset++ {
		st.dispatched(offset)
	}

	st.done(11)
	st.done(13)
	if point, moved := st.commitPoint(); point != 10 || !moved {
		t.Fatalf("expected commit point 10 while 10 is in flight, got %d moved=%v", point, moved)
	}

	st.done(10)
	if point, _ := st.commitPoint(); point != 12 {
		t.Fatalf("expected commit point 12, got %d", point)
	}

	st.done(12)
	st.done(14)
	point, moved := st.commitPoint()
	if point != 15 || !moved {
		t.Fatalf("expected commit point 15, got %d moved=%v", point, moved)
	}

	st.markCommitted(point)
	if _, moved := st.commitPoint(); moved {
		t.Fatal("commit point must not move after being committed")
	}
}

func TestOffsetTracker_DropIgnoresLateCompletions(t *testing.T) {
	tracker := newOffsetTracker()
	old := tracker.state("workflow.results", 3)
	old.dispatched(5)

	tracker.drop(map[string][]int32{"workflow.results": {3}})
	old.done(5)

	fresh := tracker.state("workflow.results", 3)
	if fresh == old {
		t.Fatal("expected a fresh state after the partition was dropped")
	}
	if got := tracker.uncommitted(nil); len(got) != 0 {
		t.Fatalf("expected nothing to commit, got %v", got)
	}
}

func TestOffsetTracker_UncommittedFiltersPartitions(t *testing.T) {
	tracker := newOffsetTracker()
	tracker.state("events", 0).dispatched(1)
	tracker.state("events", 1).dispatched(7)

	got := tracker.uncommitted(map[string][]int32{"events": {1}})
	if len(got["events"]) != 1 || got["events"][1].Offset != 7 {
		t.Fatalf("expected only partition 1 at offset 7 (in flight), got %v", got)
	}
}

func TestLaneIndex_IsStablePerKey(t *testing.T) {
	a := laneIndex([]byte("workflow-a"), 50)
	if a != laneIndex([]byte("workflow-a"), 50) {
		t.Fatal("same key must map to the same lane")
	}
	if laneIndex([]byte("x"), 1) != 0 {
		t.Fatal("single lane must always be 0")
	}
}
