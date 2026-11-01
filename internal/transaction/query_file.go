package transaction

import (
	"context"
	"database/sql"
	"io/fs"
)

// QueryFile reads an entire SQL file and executes it through Query.
func (tx *Tx) QueryFile(ctx context.Context, files fs.FS, path string, args ...any) (*sql.Rows, error) {
	sqlText, err := tx.readSQL(ctx, files, path)

	if err != nil {
		return nil, err
	}

	return tx.Query(ctx, sqlText, args...)
}
