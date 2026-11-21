package feed

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncodeDecodeCursor(t *testing.T) {
	at := time.Date(2026, 11, 21, 10, 0, 0, 123456789, time.UTC)
	id := uuid.MustParse("01960000-0000-7000-8000-000000000099")
	raw := encodeCursor(at, id)
	gotAt, gotID, err := decodeCursor(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !gotAt.Equal(at) || gotID != id {
		t.Fatalf("got %v %v", gotAt, gotID)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	if _, _, err := decodeCursor("%%%"); err == nil {
		t.Fatal("expected error")
	}
}
