package feed

import (
	"context"
	"time"
)

// WriteRepository appends feed rows in the same transaction as the source write.
type WriteRepository interface {
	Append(ctx context.Context, events []Event) error
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// ReadRepository lists the merged feed (feed_events + incident_events).
type ReadRepository interface {
	List(ctx context.Context, q ListQuery) ([]Event, error)
}
