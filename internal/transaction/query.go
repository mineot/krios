package transaction

import (
	"context"
	"database/sql"
	"fmt"
)

// Query forwards SQL and arguments to the native transaction without rewriting.
func (tx *Tx) Query(ctx context.Context, sqlText string, args ...any) (*sql.Rows, error) {
	native, err := tx.command(ctx)

	if err != nil {
		return nil, err
	}

	if err := validateSQL(sqlText); err != nil {
		return nil, err
	}

	rows, err := native.QueryContext(ctx, sqlText, args...)

	if err != nil {
		return nil, fmt.Errorf("krios: query transaction: %w", err)
	}

	return rows, nil
}
