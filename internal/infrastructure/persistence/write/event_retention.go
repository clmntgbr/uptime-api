package write

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// EventRetention purges the transactional outbox and the consumer dedup table.
// Both grow with every domain event; rows past the retention cannot be replayed.
type EventRetention struct {
	db *gorm.DB
}

func NewEventRetention(db *gorm.DB) *EventRetention {
	return &EventRetention{db: db}
}

// Published outbox rows only: unpublished rows are still owed to Kafka.
const deletePublishedOutboxSQL = `
DELETE FROM outbox_events
WHERE id IN (
	SELECT id FROM outbox_events
	WHERE published_at IS NOT NULL AND published_at < @cutoff
	LIMIT @limit
)`

const deleteProcessedEventsSQL = `
DELETE FROM processed_events
WHERE ctid IN (
	SELECT ctid FROM processed_events
	WHERE processed_at < @cutoff
	LIMIT @limit
)`

const eventRetentionBatchSize = 1000

func (r *EventRetention) DeletePublishedOutboxBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.deleteInBatches(ctx, deletePublishedOutboxSQL, cutoff)
}

func (r *EventRetention) DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.deleteInBatches(ctx, deleteProcessedEventsSQL, cutoff)
}

func (r *EventRetention) deleteInBatches(ctx context.Context, query string, cutoff time.Time) (int64, error) {
	var total int64
	for {
		res := r.db.WithContext(ctx).Exec(query, map[string]any{"cutoff": cutoff.UTC(), "limit": eventRetentionBatchSize})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < eventRetentionBatchSize {
			return total, nil
		}
	}
}
