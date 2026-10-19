// Package execute holds the internal execution contract shared by the
// connection and transaction layers: it validates the context, calls the
// underlying database/sql object and wraps the result.
package execute

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mineot/krios/internal/result"
	"github.com/mineot/krios/internal/scan"
)

// sqlExecutor is the internal execution contract satisfied by the underlying
// *sql.DB and *sql.Tx. The public methods of access.DB and transaction.Tx
// validate their own state (open or tx done) and then delegate through these
// helpers, which check the context, call the underlying database/sql object
// and apply the public wrappers.
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	_ sqlExecutor = (*sql.DB)(nil)
	_ sqlExecutor = (*sql.Tx)(nil)
)

// ExecWith validates ctx, executes the query on ex and wraps the result.
// op identifies the operation in context errors ("exec" or "tx exec").
func ExecWith(ctx context.Context, ex sqlExecutor, op, query string, args ...any) (result.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	res, err := ex.ExecContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return result.Wrap(res), nil
}

// QueryWith validates ctx, executes the query on ex and wraps the rows.
// op identifies the operation in context errors ("query" or "tx query").
func QueryWith(ctx context.Context, ex sqlExecutor, op, query string, args ...any) (*scan.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("krios: %s: %w", op, err)
	}

	r, err := ex.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	return scan.WrapRows(r), nil
}

// QueryRowWith validates ctx, executes the query on ex and wraps the row.
// Errors are deferred to Scan, including cancellation and deadline errors.
func QueryRowWith(ctx context.Context, ex sqlExecutor, query string, args ...any) scan.Row {
	if err := ctx.Err(); err != nil {
		return scan.ErrorRow(err)
	}

	return scan.WrapRow(ex.QueryRowContext(ctx, query, args...))
}
