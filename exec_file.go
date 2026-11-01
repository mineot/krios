package krios

import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/mineot/krios/internal/transaction"
)

// ExecFile reads SQL synchronously from files and executes it as Exec.
// Path and cancellation rules are the same as QueryFile. The entire file is
// forwarded unchanged, without splitting statements or translating placeholders.
func (tx *Tx) ExecFile(ctx context.Context, files fs.FS, path string, args ...any) (sql.Result, error) {
	var inner *transaction.Tx

	if tx != nil {
		inner = tx.transaction
	}

	return inner.ExecFile(ctx, files, path, args...)
}
