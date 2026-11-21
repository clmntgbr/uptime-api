package write

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/domain/feed"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type feedWriteRepository struct {
	db *gorm.DB
}

func NewFeedWriteRepository(db *gorm.DB) feed.WriteRepository {
	return &feedWriteRepository{db: db}
}

func (r *feedWriteRepository) Append(ctx context.Context, events []feed.Event) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]FeedEventModel, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("marshal feed payload: %w", err)
		}
		if payload == nil {
			payload = []byte("{}")
		}
		id := e.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		rows = append(rows, FeedEventModel{
			ID: id, UserID: e.UserID, WorkflowID: e.WorkflowID,
			RunID: e.RunID, CallID: e.CallID, IncidentID: e.IncidentID,
			Kind: string(e.Kind), Severity: string(e.Severity),
			At: e.At.UTC(), Title: e.Title, Payload: payload,
		})
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

const deleteFeedBeforeSQL = `
DELETE FROM feed_events
WHERE id IN (SELECT id FROM feed_events WHERE at < @cutoff LIMIT @limit)`

const deleteFeedBatchSize = 1000

func (r *feedWriteRepository) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		res := r.db.WithContext(ctx).Exec(deleteFeedBeforeSQL, map[string]any{
			"cutoff": cutoff.UTC(), "limit": deleteFeedBatchSize,
		})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < deleteFeedBatchSize {
			return total, nil
		}
	}
}
