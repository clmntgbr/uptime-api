// Package purge empties the database for development, keeping the users and
// the migration history.
package purge

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// KeptTables are never truncated.
var KeptTables = []string{"users", "goose_db_version"}

// Partitions are excluded: truncating a partitioned parent empties them.
const listTablesSQL = `
SELECT c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema()
  AND c.relkind IN ('r', 'p')
  AND NOT c.relispartition
  AND c.relname NOT IN @kept
ORDER BY c.relname`

// Tables returns the tables Run would truncate.
func Tables(db *gorm.DB) ([]string, error) {
	var tables []string
	if err := db.Raw(listTablesSQL, map[string]any{"kept": KeptTables}).Scan(&tables).Error; err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	return tables, nil
}

// Run truncates every table except KeptTables in a single statement. CASCADE
// is deliberately absent: if a kept table ever references a purged one, the
// purge fails instead of silently emptying the kept table.
func Run(db *gorm.DB) ([]string, error) {
	tables, err := Tables(db)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, nil
	}
	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, `"`+strings.ReplaceAll(table, `"`, `""`)+`"`)
	}
	if err := db.Exec("TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY").Error; err != nil {
		return nil, fmt.Errorf("truncate: %w", err)
	}
	return tables, nil
}
