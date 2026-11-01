package krios

import (
	"context"
	"database/sql"

	"github.com/mineot/krios/internal/transaction"
)

// Exec executes SQL that does not return rows, forwarding SQL and arguments unchanged.
// Use Query for SQL returning rows. Command errors do not trigger rollback.
// Transaction control must use DB.Begin and Tx.Commit/Rollback, not textual SQL.
func (tx *Tx) Exec(ctx context.Context, sqlText string, args ...any) (sql.Result, error) {
	var inner *transaction.Tx

	if tx != nil {
		inner = tx.transaction
	}

	return inner.Exec(ctx, sqlText, args...)
}
