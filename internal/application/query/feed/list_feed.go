package feed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	domainfeed "go-api/internal/domain/feed"

	"github.com/google/uuid"
)

type ListFeedQuery struct {
	UserID     uuid.UUID
	WorkflowID *uuid.UUID
	Kinds      []domainfeed.Kind
	Severities []domainfeed.Severity
	From       *time.Time
	To         *time.Time
	Cursor     string
	Limit      int
}

type ListFeedResult struct {
	Events     []domainfeed.Event
	NextCursor string
}

type ListFeedHandler struct {
	repo domainfeed.ReadRepository
}

func NewListFeedHandler(repo domainfeed.ReadRepository) *ListFeedHandler {
	return &ListFeedHandler{repo: repo}
}

// DefaultRetention is the feed visibility window when `from` is omitted (matches FEED_RETENTION_DAYS default).
const DefaultRetention = 30 * 24 * time.Hour

func (h *ListFeedHandler) Handle(ctx context.Context, q ListFeedQuery) (ListFeedResult, error) {
	lq := domainfeed.ListQuery{
		UserID: q.UserID, WorkflowID: q.WorkflowID,
		Kinds: q.Kinds, Severities: q.Severities,
		From: q.From, To: q.To, Limit: q.Limit,
	}
	lq.Normalize()
	// Apply the same window to feed_events and merged incident_events.
	if lq.From == nil {
		from := time.Now().UTC().Add(-DefaultRetention)
		lq.From = &from
	}
	if q.Cursor != "" {
		at, id, err := decodeCursor(q.Cursor)
		if err != nil {
			return ListFeedResult{}, err
		}
		lq.CursorAt = &at
		lq.CursorID = &id
	}
	// Fetch one extra to know if there is a next page.
	lq.Limit++
	events, err := h.repo.List(ctx, lq)
	if err != nil {
		return ListFeedResult{}, err
	}
	var next string
	if len(events) == lq.Limit {
		events = events[:lq.Limit-1]
		last := events[len(events)-1]
		next = encodeCursor(last.At, last.ID)
	}
	return ListFeedResult{Events: events, NextCursor: next}, nil
}

type cursorPayload struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func encodeCursor(at time.Time, id uuid.UUID) string {
	raw, _ := json.Marshal(cursorPayload{At: at.UTC().Format(time.RFC3339Nano), ID: id.String()})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(raw string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid cursor encoding", ErrInvalidCursor)
	}
	var p cursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid cursor payload", ErrInvalidCursor)
	}
	at, err := time.Parse(time.RFC3339Nano, p.At)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid cursor time", ErrInvalidCursor)
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: invalid cursor id", ErrInvalidCursor)
	}
	return at.UTC(), id, nil
}
