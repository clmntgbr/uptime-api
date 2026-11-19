package read

import (
	"go-api/internal/domain/paginate"

	"gorm.io/gorm"
)

func Paginate(db *gorm.DB, q paginate.PaginateQuery) (*gorm.DB, int64, error) {
	var total int64
	if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := "created_at"
	if q.SortBy != "" {
		sortBy = q.SortBy
	}

	orderBy := paginate.OrderByDesc
	if q.OrderBy == paginate.OrderByAsc {
		orderBy = paginate.OrderByAsc
	}

	// id breaks ties so pages stay stable when the sort column is not unique.
	return db.
		Order(sortBy + " " + orderBy).
		Order("id " + orderBy).
		Limit(q.Limit).
		Offset(q.Offset()), total, nil
}
