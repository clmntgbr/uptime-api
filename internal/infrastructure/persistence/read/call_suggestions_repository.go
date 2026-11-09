package read

import (
	"context"
	"fmt"
	"strings"

	domaincall "go-api/internal/domain/call"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type callSuggestionsRepository struct{ db *gorm.DB }

func NewCallSuggestionsRepository(db *gorm.DB) domaincall.SuggestionsRepository {
	return &callSuggestionsRepository{db: db}
}

func (r *callSuggestionsRepository) SearchKeys(
	ctx context.Context, userID uuid.UUID, field domaincall.MapField, search string, limit int,
) ([]string, error) {
	col, err := mapColumn(field)
	if err != nil {
		return nil, err
	}
	search = strings.TrimSpace(search)
	params := map[string]any{
		"user_id": userID,
		"search":  search,
		"limit":   limit,
	}
	var rows []string
	// Column name is switched from an enum — never interpolated from request input.
	q := fmt.Sprintf(`
SELECT DISTINCT kv.key
FROM calls c
JOIN workflows w ON w.id = c.workflow_id
CROSS JOIN LATERAL jsonb_each_text(c.%s) AS kv(key, value)
WHERE w.user_id = @user_id
  AND w.deleted_at IS NULL
  AND kv.key <> ''
  AND (@search = '' OR kv.key ILIKE '%%' || @search || '%%')
ORDER BY kv.key
LIMIT @limit`, col)
	if err := r.db.WithContext(ctx).Raw(q, params).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []string{}
	}
	return rows, nil
}

func (r *callSuggestionsRepository) SearchValues(
	ctx context.Context, userID uuid.UUID, field domaincall.MapField, search string, limit int,
) ([]string, error) {
	col, err := mapColumn(field)
	if err != nil {
		return nil, err
	}
	search = strings.TrimSpace(search)
	params := map[string]any{
		"user_id": userID,
		"search":  search,
		"limit":   limit,
	}
	var rows []string
	q := fmt.Sprintf(`
SELECT DISTINCT kv.value
FROM calls c
JOIN workflows w ON w.id = c.workflow_id
CROSS JOIN LATERAL jsonb_each_text(c.%s) AS kv(key, value)
WHERE w.user_id = @user_id
  AND w.deleted_at IS NULL
  AND kv.value <> ''
  AND (@search = '' OR kv.value ILIKE '%%' || @search || '%%')
ORDER BY kv.value
LIMIT @limit`, col)
	if err := r.db.WithContext(ctx).Raw(q, params).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []string{}
	}
	return rows, nil
}

func mapColumn(field domaincall.MapField) (string, error) {
	switch field {
	case domaincall.MapHeaders:
		return "headers", nil
	case domaincall.MapQuery:
		return "query", nil
	default:
		return "", fmt.Errorf("unknown call map field %q", field)
	}
}
