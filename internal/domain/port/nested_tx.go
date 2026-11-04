package port

import "context"

// NestedTransaction supports SAVEPOINT / ROLLBACK TO inside an open transaction.
type NestedTransaction interface {
	SavePoint(ctx context.Context, name string) error
	RollbackTo(ctx context.Context, name string) error
}
