package write

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type nestedTx struct{}

func NewNestedTransaction() *nestedTx { return &nestedTx{} }

func (nestedTx) SavePoint(ctx context.Context, name string) error {
	db, ok := ctx.Value(txCtxKey{}).(*gorm.DB)
	if !ok || db == nil {
		return fmt.Errorf("no transaction in context")
	}
	return db.SavePoint(name).Error
}

func (nestedTx) RollbackTo(ctx context.Context, name string) error {
	db, ok := ctx.Value(txCtxKey{}).(*gorm.DB)
	if !ok || db == nil {
		return fmt.Errorf("no transaction in context")
	}
	return db.RollbackTo(name).Error
}
