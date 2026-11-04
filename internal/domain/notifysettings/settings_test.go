package notifysettings

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeAndValidateEmails(t *testing.T) {
	got := NormalizeEmails([]string{"  A@B.com ", "a@b.com", "", "c@d.com"})
	if len(got) != 2 || got[0] != "a@b.com" || got[1] != "c@d.com" {
		t.Fatalf("got %#v", got)
	}
	if err := ValidateEmails(got); err != nil {
		t.Fatal(err)
	}
}

func TestAllowsRunTransition(t *testing.T) {
	s := DefaultSettings(uuid.New(), uuid.New(), "a@b.com")
	s.RunsEnabled = true
	s.RunStatuses = []RunStatus{RunFailed, RunSuccess}
	s.RunMode = RunModeTransition
	if s.AllowsRun("failed", "") {
		t.Fatal("no previous status should not notify in transition mode")
	}
	if !s.AllowsRun("failed", "success") {
		t.Fatal("success→failed must notify")
	}
	if s.AllowsRun("failed", "failed") {
		t.Fatal("same status must not notify")
	}
}

func TestDefaultDisablesRuns(t *testing.T) {
	s := DefaultSettings(uuid.New(), uuid.New(), "a@b.com")
	if s.RunsEnabled || !s.IncidentsEnabled || !s.TLSEnabled || len(s.Emails) != 1 {
		t.Fatalf("%+v", s)
	}
}
