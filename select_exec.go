package krios

import (
	"context"
	"database/sql"
)

// Exec compiles and queries using ctx. The caller must consume, check and close
// Rows before finalizing the transaction. Command errors do not trigger rollback.
func (q SelectBuilder) Exec(ctx context.Context) (*sql.Rows, error) {
	return q.query.Exec(ctx)
}
