package read

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/domain/feed"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type feedQueryRepository struct {
	db *gorm.DB
}

func NewFeedQueryRepository(db *gorm.DB) feed.ReadRepository {
	return &feedQueryRepository{db: db}
}

// listFeedSQL merges feed_events with projected incident_events (opened/reopened/ack/resolved).
// availability_low transitions are exposed as availability.breached / availability.recovered.
const listFeedSQL = `
SELECT * FROM (
  SELECT
    f.id,
    f.user_id,
    f.workflow_id,
    f.run_id,
    f.call_id,
    f.incident_id,
    f.kind,
    f.severity,
    f.at,
    f.title,
    f.payload
  FROM feed_events f
  WHERE f.user_id = @user_id
    AND (@workflow_id::uuid IS NULL OR f.workflow_id = @workflow_id)
    AND (@from_at::timestamptz IS NULL OR f.at >= @from_at)
    AND (@to_at::timestamptz IS NULL OR f.at <= @to_at)
    AND (CARDINALITY(@kinds::text[]) = 0 OR f.kind = ANY(@kinds))
    AND (CARDINALITY(@severities::text[]) = 0 OR f.severity = ANY(@severities))
    AND (
      @cursor_at::timestamptz IS NULL
      OR (f.at, f.id) < (@cursor_at, @cursor_id)
    )

  UNION ALL

  SELECT
    ie.id,
    i.user_id,
    i.workflow_id,
    ie.run_id,
    i.call_id,
    ie.incident_id,
    CASE
      WHEN i.kind = 'availability_low' AND ie.type IN ('opened', 'reopened') THEN 'availability.breached'
      WHEN i.kind = 'availability_low' AND ie.type = 'resolved' THEN 'availability.recovered'
      WHEN ie.type = 'opened' THEN 'incident.opened'
      WHEN ie.type = 'reopened' THEN 'incident.reopened'
      WHEN ie.type = 'acknowledged' THEN 'incident.acknowledged'
      WHEN ie.type = 'resolved' THEN 'incident.resolved'
      ELSE 'incident.' || ie.type
    END AS kind,
    i.severity,
    ie.at,
    CASE
      WHEN i.kind = 'availability_low' AND ie.type IN ('opened', 'reopened') THEN 'Availability breached'
      WHEN i.kind = 'availability_low' AND ie.type = 'resolved' THEN 'Availability recovered'
      WHEN ie.type = 'opened' THEN 'Incident opened (' || i.kind || ')'
      WHEN ie.type = 'reopened' THEN 'Incident reopened (' || i.kind || ')'
      WHEN ie.type = 'acknowledged' THEN 'Incident acknowledged (' || i.kind || ')'
      WHEN ie.type = 'resolved' THEN 'Incident resolved (' || i.kind || ')'
      ELSE 'Incident ' || ie.type
    END AS title,
    ie.payload
  FROM incident_events ie
  INNER JOIN incidents i ON i.id = ie.incident_id
  WHERE i.user_id = @user_id
    AND ie.type IN ('opened', 'reopened', 'acknowledged', 'resolved')
    AND (@workflow_id::uuid IS NULL OR i.workflow_id = @workflow_id)
    AND (@from_at::timestamptz IS NULL OR ie.at >= @from_at)
    AND (@to_at::timestamptz IS NULL OR ie.at <= @to_at)
    AND (
      CARDINALITY(@kinds::text[]) = 0
      OR (
        CASE
          WHEN i.kind = 'availability_low' AND ie.type IN ('opened', 'reopened') THEN 'availability.breached'
          WHEN i.kind = 'availability_low' AND ie.type = 'resolved' THEN 'availability.recovered'
          WHEN ie.type = 'opened' THEN 'incident.opened'
          WHEN ie.type = 'reopened' THEN 'incident.reopened'
          WHEN ie.type = 'acknowledged' THEN 'incident.acknowledged'
          WHEN ie.type = 'resolved' THEN 'incident.resolved'
          ELSE 'incident.' || ie.type
        END
      ) = ANY(@kinds)
    )
    AND (CARDINALITY(@severities::text[]) = 0 OR i.severity = ANY(@severities))
    AND (
      @cursor_at::timestamptz IS NULL
      OR (ie.at, ie.id) < (@cursor_at, @cursor_id)
    )
) AS feed
ORDER BY at DESC, id DESC
LIMIT @limit
`

type feedListRow struct {
	ID         uuid.UUID    `gorm:"column:id"`
	UserID     uuid.UUID    `gorm:"column:user_id"`
	WorkflowID *uuid.UUID   `gorm:"column:workflow_id"`
	RunID      *uuid.UUID   `gorm:"column:run_id"`
	CallID     *uuid.UUID   `gorm:"column:call_id"`
	IncidentID *uuid.UUID   `gorm:"column:incident_id"`
	Kind       string       `gorm:"column:kind"`
	Severity   string       `gorm:"column:severity"`
	At         time.Time    `gorm:"column:at"`
	Title      string       `gorm:"column:title"`
	Payload    dbtype.JSONB `gorm:"column:payload"`
}

func (r *feedQueryRepository) List(ctx context.Context, q feed.ListQuery) ([]feed.Event, error) {
	q.Normalize()
	kinds := make([]string, 0, len(q.Kinds))
	for _, k := range q.Kinds {
		kinds = append(kinds, string(k))
	}
	sevs := make([]string, 0, len(q.Severities))
	for _, s := range q.Severities {
		sevs = append(sevs, string(s))
	}
	params := map[string]any{
		"user_id":     q.UserID,
		"workflow_id": q.WorkflowID,
		"from_at":     q.From,
		"to_at":       q.To,
		"kinds":       pq.StringArray(kinds),
		"severities":  pq.StringArray(sevs),
		"cursor_at":   q.CursorAt,
		"cursor_id":   q.CursorID,
		"limit":       q.Limit,
	}
	var rows []feedListRow
	if err := r.db.WithContext(ctx).Raw(listFeedSQL, params).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]feed.Event, 0, len(rows))
	for _, row := range rows {
		payload := map[string]any{}
		if len(row.Payload) > 0 {
			if err := json.Unmarshal(row.Payload, &payload); err != nil {
				return nil, fmt.Errorf("unmarshal feed payload: %w", err)
			}
		}
		out = append(out, feed.Event{
			ID: row.ID, UserID: row.UserID, WorkflowID: row.WorkflowID,
			RunID: row.RunID, CallID: row.CallID, IncidentID: row.IncidentID,
			Kind: feed.Kind(row.Kind), Severity: feed.Severity(row.Severity),
			At: row.At.UTC(), Title: row.Title, Payload: payload,
		})
	}
	return out, nil
}
