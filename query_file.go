package krios

import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/mineot/krios/internal/transaction"
)

// QueryFile reads SQL synchronously from files and executes it as Query.
// The path must satisfy fs.ValidPath and must not be ".". Reading is not cached
// and cannot be interrupted by ctx; ctx is checked again before executing SQL.
func (tx *Tx) QueryFile(ctx context.Context, files fs.FS, path string, args ...any) (*sql.Rows, error) {
	var inner *transaction.Tx

	if tx != nil {
		inner = tx.transaction
	}

	return inner.QueryFile(ctx, files, path, args...)
}
