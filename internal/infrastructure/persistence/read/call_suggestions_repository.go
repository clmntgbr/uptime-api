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
		"search":  escapeLike(search),
		"limit":   limit,
	}
	var rows []string
	// Column name is switched from an enum — never interpolated from request input.
	q := fmt.Sprintf(`
SELECT DISTINCT kv.key
FROM calls c
JOIN workflows w ON w.id = c.workflow_id
CROSS JOIN LATERAL jsonb_each_text(%s) AS kv(key, value)
WHERE w.user_id = @user_id
  AND w.deleted_at IS NULL
  AND kv.key <> ''
%s
ORDER BY kv.key
LIMIT @limit`, mapExpr(col), searchClause("kv.key", search))
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
		"search":  escapeLike(search),
		"limit":   limit,
	}
	var rows []string
	q := fmt.Sprintf(`
SELECT DISTINCT kv.value
FROM calls c
JOIN workflows w ON w.id = c.workflow_id
CROSS JOIN LATERAL jsonb_each_text(%s) AS kv(key, value)
WHERE w.user_id = @user_id
  AND w.deleted_at IS NULL
  AND kv.value <> ''
%s
ORDER BY kv.value
LIMIT @limit`, mapExpr(col), searchClause("kv.value", search))
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

// mapExpr guards jsonb_each_text: a non-object value (legacy row) yields no rows
// instead of failing the whole suggestion query for the user.
func mapExpr(col string) string {
	return fmt.Sprintf("CASE WHEN jsonb_typeof(c.%[1]s) = 'object' THEN c.%[1]s ELSE '{}'::jsonb END", col)
}

// searchClause renders the optional search predicate in Go instead of
// "(@search = ” OR ...)", which the planner cannot simplify.
func searchClause(column, search string) string {
	if search == "" {
		return ""
	}
	return fmt.Sprintf("  AND %s ILIKE '%%' || @search || '%%'", column)
}
