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

func TestDefaultDisablesNothingExceptEmptyRuns(t *testing.T) {
	s := DefaultSettings(uuid.New(), uuid.New(), "a@b.com")
	if !s.IncidentsEnabled || !s.TLSEnabled || len(s.Emails) != 1 {
		t.Fatalf("%+v", s)
	}
	if !s.AllowsIncident() || !s.AllowsTLS() {
		t.Fatal("defaults should allow incident and TLS mail")
	}
}
