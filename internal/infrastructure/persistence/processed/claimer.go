package processed

import (
	"context"
	"time"

	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// claimer marks (event_id, handler) with ON CONFLICT DO NOTHING. Unlike a
// failing INSERT, it leaves the surrounding transaction usable, so it can
// guard side effects written in that same transaction.
type claimer struct {
	db *gorm.DB
}

func NewTxClaimer(db *gorm.DB) port.ProcessedEventRepository {
	return &claimer{db: db}
}

func (c *claimer) MarkProcessed(ctx context.Context, eventID, handlerName string) (bool, error) {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return false, err
	}
	res := write.DBWithContext(ctx, c.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&ProcessedEvent{EventID: id, HandlerName: handlerName, ProcessedAt: time.Now().UTC()})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
