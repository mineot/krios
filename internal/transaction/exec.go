package transaction

import (
	"context"
	"database/sql"
	"fmt"
)

// Exec forwards SQL and arguments to the native transaction without rewriting.
func (tx *Tx) Exec(ctx context.Context, sqlText string, args ...any) (sql.Result, error) {
	native, err := tx.command(ctx)

	if err != nil {
		return nil, err
	}

	if err := validateSQL(sqlText); err != nil {
		return nil, err
	}

	result, err := native.ExecContext(ctx, sqlText, args...)

	if err != nil {
		return nil, fmt.Errorf("krios: exec transaction: %w", err)
	}

	return result, nil
}
