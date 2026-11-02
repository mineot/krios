package selectquery

import (
	"context"
	"database/sql"
)

// Exec validates transaction/context before compiling and querying.
// Query rechecks lifecycle state; the caller owns the returned rows.
func (q Builder) Exec(ctx context.Context) (*sql.Rows, error) {
	if err := q.tx.CheckCommand(ctx); err != nil {
		return nil, err
	}
	sqlText, args, err := q.Build()
	if err != nil {
		return nil, err
	}
	return q.tx.Query(ctx, sqlText, args...)
}
