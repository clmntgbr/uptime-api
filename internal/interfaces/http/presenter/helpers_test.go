package presenter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOptionalHelpers_NilForEmpty(t *testing.T) {
	if optionalNonEmptyString("") != nil {
		t.Fatal("empty string must map to nil")
	}
	if got := optionalNonEmptyString("x"); got == nil || *got != "x" {
		t.Fatalf("non-empty string: got %v", got)
	}
	if optionalTime(time.Time{}) != nil {
		t.Fatal("zero time must map to nil")
	}
	if optionalUUIDString(nil) != nil {
		t.Fatal("nil uuid pointer must map to nil")
	}
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if got := optionalUUIDString(&id); got == nil || *got != id.String() {
		t.Fatalf("uuid: got %v", got)
	}
}

func TestNonNilMap_SerialisesAsObject(t *testing.T) {
	out, err := json.Marshal(nonNilMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "{}" {
		t.Fatalf("nil map must serialise as {}, got %s", out)
	}
}

func TestRunListResponse_EmptySerialisesAsArray(t *testing.T) {
	out, err := json.Marshal(NewRunListResponseFromViews(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[]" {
		t.Fatalf("empty list must serialise as [], got %s", out)
	}
}
