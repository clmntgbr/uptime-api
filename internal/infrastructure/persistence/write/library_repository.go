package write

import (
	"context"
	"errors"

	domainlibrary "go-api/internal/domain/library"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type libraryRepository struct {
	db *gorm.DB
}

func NewLibraryWriteRepository(db *gorm.DB) domainlibrary.WriteRepository {
	return &libraryRepository{db: db}
}

func (r *libraryRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

func (r *libraryRepository) Save(ctx context.Context, item *domainlibrary.Item) error {
	model, err := newLibraryItemModel(*item)
	if err != nil {
		return err
	}
	return DBWithContext(ctx, r.db).Save(&model).Error
}

func (r *libraryRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	res := DBWithContext(ctx, r.db).
		Where("user_id = ? AND id = ?", userID, id).
		Delete(&LibraryItemModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domainlibrary.ErrNotFound
	}
	return nil
}

func (r *libraryRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*domainlibrary.Item, error) {
	var model LibraryItemModel
	err := DBWithContext(ctx, r.db).Where("user_id = ? AND id = ?", userID, id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	item, err := toLibraryItem(model)
	if err != nil {
		return nil, err
	}
	return &item, nil
}
