package expiry

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PolicyRepository interface {
	// Get returns nil, nil when no row (caller applies defaults).
	Get(ctx context.Context, workflowID uuid.UUID, t Type) (*Policy, error)
	ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) ([]Policy, error)
	Upsert(ctx context.Context, p Policy) error
}

type ItemRepository interface {
	Upsert(ctx context.Context, item Item) (Item, error)
	ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID, t Type) ([]Item, error)
	// ListAllForScan returns every item for the daily scheduler pass.
	ListAllForScan(ctx context.Context) ([]Item, error)
	// ListExpiringBetween returns items whose expiry falls in [from, to].
	// The daily expiry pass only needs this window: see expiry.ScanWindow.
	ListExpiringBetween(ctx context.Context, from, to time.Time) ([]Item, error)
}

type AlertRepository interface {
	// Claim inserts alert rows; returns thresholds that were newly claimed.
	Claim(ctx context.Context, itemID uuid.UUID, fingerprint string, thresholds []int) ([]int, error)
	// HadOtherFingerprint is true when an alert exists for this item under a
	// different certificate fingerprint (or a dated expired claim of another cert).
	HadOtherFingerprint(ctx context.Context, itemID uuid.UUID, fingerprint string) (bool, error)
}

type ItemView struct {
	Item
	DaysLeft          int
	State             ItemState
	NextThresholdDays *int
	Issuer            string
}
