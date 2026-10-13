package event

import (
	"strings"

	"github.com/google/uuid"
)

// DeterministicID builds a stable UUID from the given parts (useful for
// idempotent synthetic events).
func DeterministicID(parts ...string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(strings.Join(parts, ":"))).String()
}
