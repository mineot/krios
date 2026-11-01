package transaction

import (
	"context"
	"database/sql"
	"io/fs"
)

// ExecFile reads an entire SQL file and executes it through Exec.
func (tx *Tx) ExecFile(ctx context.Context, files fs.FS, path string, args ...any) (sql.Result, error) {
	sqlText, err := tx.readSQL(ctx, files, path)

	if err != nil {
		return nil, err
	}

	return tx.Exec(ctx, sqlText, args...)
}
