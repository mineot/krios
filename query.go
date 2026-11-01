package krios

import (
	"context"
	"database/sql"

	"github.com/mineot/krios/internal/transaction"
)

// Query executes SQL with native placeholders and separate arguments.
// The caller must consume, check and close Rows before finalizing the transaction.
// SQL and arguments are forwarded unchanged; command errors do not trigger rollback.
func (tx *Tx) Query(ctx context.Context, sqlText string, args ...any) (*sql.Rows, error) {
	var inner *transaction.Tx

	if tx != nil {
		inner = tx.transaction
	}

	return inner.Query(ctx, sqlText, args...)
}
