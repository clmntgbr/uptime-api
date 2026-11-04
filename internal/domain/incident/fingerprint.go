package incident

import (
	"github.com/google/uuid"
)

// Fingerprint builds workflow:kind:subject where subject is call UUID or
// a stable observation key (host:port for TLS). Empty call uses uuid.Nil.
func Fingerprint(workflowID uuid.UUID, kind Kind, subject string) string {
	if subject == "" {
		subject = uuid.Nil.String()
	}
	return workflowID.String() + ":" + string(kind) + ":" + subject
}

func CallSubject(callID uuid.UUID) string {
	if callID == uuid.Nil {
		return uuid.Nil.String()
	}
	return callID.String()
}
