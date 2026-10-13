package certificate

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Certificate is the latest TLS certificate observed for a host by the user's
// calls. It is the base of a future expiry inventory.
type Certificate struct {
	UserID         uuid.UUID
	Host           string
	Port           int
	Subject        string
	Issuer         string
	SerialNumber   string
	ExpiresAt      time.Time
	ChainExpiresAt time.Time
	ChainValid     bool
	NameMatch      bool
	SeenAt         time.Time
}

type Repository interface {
	// Upsert keeps one row per (user, host, port); an older observation never
	// overwrites a newer one.
	Upsert(ctx context.Context, certificates []Certificate) error
}
