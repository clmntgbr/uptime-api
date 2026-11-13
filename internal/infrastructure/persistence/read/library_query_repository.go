package read

import (
	"context"
	"strings"
	"time"

	domainlibrary "go-api/internal/domain/library"
	"go-api/internal/domain/paginate"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var librarySortColumns = map[string]string{
	"name":       "name",
	"createdAt":  "created_at",
	"created_at": "created_at",
	"updatedAt":  "updated_at",
	"updated_at": "updated_at",
}

type libraryQueryRepository struct {
	db *gorm.DB
}

func NewLibraryQueryRepository(db *gorm.DB) domainlibrary.ReadRepository {
	return &libraryQueryRepository{db: db}
}

type libraryListRow struct {
	ID          uuid.UUID `gorm:"column:id"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Method      string    `gorm:"column:method"`
	URL         string    `gorm:"column:url"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (r *libraryQueryRepository) List(ctx context.Context, userID uuid.UUID, q paginate.PaginateQuery) ([]domainlibrary.ListView, int64, error) {
	column, ok := librarySortColumns[q.SortBy]
	if !ok {
		column = "created_at"
	}
	q.SortBy = column

	base := r.db.WithContext(ctx).
		Table("library_items").
		Where("user_id = ?", userID)
	if search := strings.TrimSpace(q.Search); search != "" {
		base = base.Where("name ILIKE ?", "%"+escapeLike(search)+"%")
	}

	scoped, total, err := Paginate(base, q)
	if err != nil {
		return nil, 0, err
	}
	var rows []libraryListRow
	if err := scoped.Select("id, name, description, payload->>'method' AS method, payload->>'url' AS url, created_at, updated_at").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	views := make([]domainlibrary.ListView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domainlibrary.ListView{
			ID: row.ID, Name: row.Name, Description: row.Description,
			Method: row.Method, URL: row.URL, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return views, total, nil
}
