package outbox

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/persistence/dbtype"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) port.OutboxRepository {
	return &repository{db: db}
}

func (r *repository) StoreEvents(ctx context.Context, events []event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	rows := make([]OutboxEvent, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		eventID, err := uuid.Parse(e.EventID())
		if err != nil {
			return err
		}
		rows = append(rows, OutboxEvent{
			ID:          eventID,
			AggregateID: e.AggregateID(),
			EventType:   e.EventType(),
			Payload:     dbtype.JSONB(payload),
			CreatedAt:   e.OccurredAt(),
		})
	}

	return write.DBWithContext(ctx, r.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}

// claimLease is how long a fetched batch stays reserved for one relay. If the
// publisher crashes first, the rows become claimable again after this delay.
const claimLease = 30 * time.Second

// FetchUnpublished claims up to limit unpublished rows in one statement.
// FOR UPDATE SKIP LOCKED makes concurrent relays skip rows another relay is
// already claiming, so replicas do not publish the same event twice.
func (r *repository) FetchUnpublished(ctx context.Context, limit int) ([]port.OutboxMessage, error) {
	var rows []OutboxEvent
	err := r.db.WithContext(ctx).Raw(`
		UPDATE outbox_events
		SET claimed_until = NOW() + make_interval(secs => ?)
		WHERE id IN (
			SELECT id FROM outbox_events
			WHERE published_at IS NULL
			  AND (claimed_until IS NULL OR claimed_until < NOW())
			ORDER BY created_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, aggregate_id, event_type, payload, created_at, published_at`,
		claimLease.Seconds(), limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// RETURNING does not guarantee order: publish oldest first.
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })

	messages := make([]port.OutboxMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, port.OutboxMessage{
			ID:          row.ID,
			AggregateID: row.AggregateID,
			EventType:   row.EventType,
			Payload:     []byte(row.Payload),
			CreatedAt:   row.CreatedAt,
			PublishedAt: row.PublishedAt,
		})
	}
	return messages, nil
}

func (r *repository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&OutboxEvent{}).
		Where("id IN ?", ids).
		Update("published_at", now).Error
}
